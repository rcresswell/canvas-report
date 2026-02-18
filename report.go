// ABOUTME: Generates formatted reports of Canvas assignments and grades.
// ABOUTME: Shows missing, upcoming, and week-ahead assignments plus current grades by grading period.

package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
	"golang.org/x/term"
)

const oneMonthAgo = 30 * 24 * time.Hour

type Report struct {
	client  *CanvasClient
	showAll bool
}

type columnWidths struct {
	subject    int
	assignment int
	due        int
	pts        int
	impact     int
	status     int
}

func calculateColumnWidths(assignments ...[]EnrichedAssignment) columnWidths {
	// Fixed widths for predictable columns
	const (
		dueWidth    = 18 // "thu 12/18 11pm" + padding
		ptsWidth    = 5
		impactWidth = 13 // "+10.0/-10.0%" or "+100/-100 pts"
		statusWidth = 3
		// Table overhead: 6 column separators (│) + padding (2 per col) = ~20 chars
		overhead = 20
		minWidth = 80
	)

	termWidth := 120 // default
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		termWidth = w
	}
	if termWidth < minWidth {
		termWidth = minWidth
	}

	// Find actual max widths from content
	maxSubject := 0
	maxAssignment := 0
	for _, list := range assignments {
		for _, a := range list {
			if len(a.CourseName) > maxSubject {
				maxSubject = len(a.CourseName)
			}
			if len(a.Name) > maxAssignment {
				maxAssignment = len(a.Name)
			}
		}
	}

	// Available space for subject + assignment
	flexible := termWidth - dueWidth - ptsWidth - impactWidth - statusWidth - overhead

	// If everything fits, use actual widths
	if maxSubject+maxAssignment <= flexible {
		return columnWidths{
			subject:    maxSubject,
			assignment: maxAssignment,
			due:        dueWidth,
			pts:        ptsWidth,
			impact:     impactWidth,
			status:     statusWidth,
		}
	}

	// Otherwise, scale proportionally based on actual content needs
	total := maxSubject + maxAssignment
	subjectWidth := flexible * maxSubject / total
	assignmentWidth := flexible - subjectWidth

	return columnWidths{
		subject:    subjectWidth,
		assignment: assignmentWidth,
		due:        dueWidth,
		pts:        ptsWidth,
		impact:     impactWidth,
		status:     statusWidth,
	}
}

type AssignmentImpact struct {
	Gain       float64 // Max improvement if 100% (positive number)
	Loss       float64 // Loss if 0% (positive number)
	IsWeighted bool    // Determines display format (% vs pts)
}

type EnrichedAssignment struct {
	Name           string
	CourseName     string
	CategoryName   string // Weighted category (e.g., "Summative", "Formative")
	DueAt          time.Time
	PointsPossible *float64
	Submission     *Submission
	Status         string
	Impact         *AssignmentImpact
}

type studentData struct {
	name             string
	missing          []EnrichedAssignment
	upcoming         []EnrichedAssignment
	weekAhead        []EnrichedAssignment
	upcomingPending  int
	weekAheadPending int
	grades           []periodGrades
}

type periodGrades struct {
	period GradingPeriod
	grades []CourseGrade
}

type CourseGrade struct {
	CourseName     string
	Points         float64
	PointsPossible float64
	Percent        float64
	Weighted       bool
	Categories     []CategoryGrade
}

type CategoryGrade struct {
	Name           string
	Points         float64
	PointsPossible float64
	Percent        float64
	Weight         float64
}

func NewReport(client *CanvasClient, showAll bool) *Report {
	return &Report{client: client, showAll: showAll}
}

func (r *Report) Generate() error {
	observees, err := r.client.Observees()
	if err != nil {
		return err
	}

	if len(observees) == 0 {
		fmt.Println("No observed students found. Make sure you have parent observer access set up in Canvas.")
		return nil
	}

	// Fetch all student data first
	var allStudents []studentData
	var allAssignments [][]EnrichedAssignment

	for _, student := range observees {
		data, err := r.fetchStudentData(student)
		if err != nil {
			return err
		}
		allStudents = append(allStudents, data)
		allAssignments = append(allAssignments, data.missing, data.upcoming, data.weekAhead)
	}

	// Calculate column widths across ALL students' data
	colWidths := calculateColumnWidths(allAssignments...)

	// Print all reports with consistent widths
	// Table width = columns + separators (│) + padding (1 char each side per column)
	tableWidth := colWidths.subject + colWidths.assignment + colWidths.due + colWidths.pts + colWidths.impact + colWidths.status + 19

	for i, data := range allStudents {
		if i > 0 {
			fmt.Println()
			fmt.Println(strings.Repeat("═", tableWidth))
		}
		r.printReport(data, colWidths)
	}

	return nil
}

func (r *Report) fetchStudentData(student Observee) (studentData, error) {
	name := student.Name
	if name == "" {
		name = student.ShortName
	}
	if name == "" {
		name = "Unknown Student"
	}

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
	s.Prefix = fmt.Sprintf("[")
	s.Suffix = fmt.Sprintf("] %s: fetching courses...", name)
	s.Start()

	courses, err := r.client.Courses(student.ID)
	if err != nil {
		s.Stop()
		return studentData{}, err
	}

	s.Suffix = fmt.Sprintf("] %s: 0/%d courses...", name, len(courses))

	assignments := r.fetchAllAssignments(courses, student.ID, s, name)

	s.Suffix = fmt.Sprintf("] %s: fetching grades...", name)
	grades := r.fetchAllGrades(courses, student.ID)

	s.Stop()
	gradeCount := 0
	for _, pg := range grades {
		gradeCount += len(pg.grades)
	}
	fmt.Fprintf(os.Stderr, "[✔] %s: %d courses, %d assignments, %d grades\n", name, len(courses), len(assignments), gradeCount)

	missing := r.missingAssignments(assignments)
	upcoming := r.upcomingAssignments(assignments)
	weekAhead := r.weekAheadAssignments(assignments)

	return studentData{
		name:             name,
		missing:          missing,
		upcoming:         upcoming,
		weekAhead:        weekAhead,
		upcomingPending:  countPending(upcoming),
		weekAheadPending: countPending(weekAhead),
		grades:           grades,
	}, nil
}

func (r *Report) fetchAllAssignments(courses []Course, studentID int, s *spinner.Spinner, studentName string) []EnrichedAssignment {
	var assignments []EnrichedAssignment
	var errors []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	completed := 0
	total := len(courses)

	for _, course := range courses {
		wg.Add(1)
		go func(c Course) {
			defer wg.Done()

			courseAssignments, err := r.fetchCourseAssignments(c, studentID)

			mu.Lock()
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", c.Name, err))
			}
			assignments = append(assignments, courseAssignments...)
			completed++
			s.Suffix = fmt.Sprintf("] %s: %d/%d courses...", studentName, completed, total)
			mu.Unlock()
		}(course)
	}

	wg.Wait()

	for _, e := range errors {
		fmt.Fprintf(os.Stderr, "  warning: %s\n", e)
	}

	return assignments
}

func (r *Report) fetchCourseAssignments(course Course, studentID int) ([]EnrichedAssignment, error) {
	var result []EnrichedAssignment

	rawAssignments, err := r.client.Assignments(course.ID)
	if err != nil {
		return result, fmt.Errorf("fetching assignments: %w", err)
	}

	rawSubmissions, err := r.client.Submissions(course.ID, studentID)
	if err != nil {
		return result, fmt.Errorf("fetching submissions: %w", err)
	}

	// Fetch assignment groups for impact calculation
	groups, err := r.client.AssignmentGroups(course.ID)
	if err != nil {
		groups = nil // Continue without impact if groups fail
	}

	// Get current grade and grading period for impact calculation
	var currentOverall float64
	var currentPeriod *GradingPeriod
	weighted := isWeightedGrading(groups)
	if groups != nil {
		periods, _ := r.client.GradingPeriods(course.ID)
		currentPeriod = currentGradingPeriod(periods)
		if currentPeriod != nil {
			periodID := fmt.Sprintf("%v", currentPeriod.ID)
			enrollments, _ := r.client.Enrollments(course.ID, studentID, periodID)
			if len(enrollments) > 0 && enrollments[0].Grades.CurrentScore != nil {
				currentOverall = *enrollments[0].Grades.CurrentScore
			}
		}
	}

	// Calculate impacts
	var impacts map[int]*AssignmentImpact
	if groups != nil {
		impacts = calculateAssignmentImpacts(groups, rawSubmissions, currentOverall, weighted, currentPeriod)
	}

	// Build map of assignment ID to category name (only for weighted courses)
	categoryByAssignment := make(map[int]string)
	if weighted && groups != nil {
		for _, group := range groups {
			for _, a := range group.Assignments {
				categoryByAssignment[a.ID] = group.Name
			}
		}
	}

	submissionsByID := make(map[int]*Submission)
	for i := range rawSubmissions {
		submissionsByID[rawSubmissions[i].AssignmentID] = &rawSubmissions[i]
	}

	courseName := course.Name
	if courseName == "" {
		courseName = "Unknown Course"
	}

	for _, a := range rawAssignments {
		if a.DueAt == nil {
			continue
		}

		result = append(result, EnrichedAssignment{
			Name:           a.Name,
			CourseName:     courseName,
			CategoryName:   categoryByAssignment[a.ID],
			DueAt:          *a.DueAt,
			PointsPossible: a.PointsPossible,
			Submission:     submissionsByID[a.ID],
			Impact:         impacts[a.ID],
		})
	}

	return result, nil
}

func (r *Report) fetchAllGrades(courses []Course, studentID int) []periodGrades {
	type courseGradeResult struct {
		period *GradingPeriod
		grade  *CourseGrade
	}

	var results []courseGradeResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, course := range courses {
		wg.Add(1)
		go func(c Course) {
			defer wg.Done()

			period, grade := r.fetchCourseGrade(c, studentID)

			mu.Lock()
			if period != nil && grade != nil {
				results = append(results, courseGradeResult{period: period, grade: grade})
			}
			mu.Unlock()
		}(course)
	}

	wg.Wait()

	// Group by grading period (using title as key since ID can be string or int)
	periodMap := make(map[string]*periodGrades)
	for _, res := range results {
		key := res.period.Title
		if pg, ok := periodMap[key]; ok {
			pg.grades = append(pg.grades, *res.grade)
		} else {
			periodMap[key] = &periodGrades{
				period: *res.period,
				grades: []CourseGrade{*res.grade},
			}
		}
	}

	// Convert map to slice and sort by period start date
	var grouped []periodGrades
	for _, pg := range periodMap {
		// Sort grades by course name within each period
		sort.Slice(pg.grades, func(i, j int) bool {
			return pg.grades[i].CourseName < pg.grades[j].CourseName
		})
		grouped = append(grouped, *pg)
	}

	// Sort periods by start date
	sort.Slice(grouped, func(i, j int) bool {
		if grouped[i].period.StartDate == nil {
			return true
		}
		if grouped[j].period.StartDate == nil {
			return false
		}
		return grouped[i].period.StartDate.Before(*grouped[j].period.StartDate)
	})

	return grouped
}

func (r *Report) fetchCourseGrade(course Course, studentID int) (*GradingPeriod, *CourseGrade) {
	periods, err := r.client.GradingPeriods(course.ID)
	if err != nil {
		return nil, nil
	}

	current := currentGradingPeriod(periods)
	if current == nil {
		return nil, nil
	}

	// Convert grading period ID to string for API call
	periodID := fmt.Sprintf("%v", current.ID)
	enrollments, err := r.client.Enrollments(course.ID, studentID, periodID)
	if err != nil || len(enrollments) == 0 {
		return nil, nil
	}

	enrollment := enrollments[0]
	if enrollment.Grades.CurrentScore == nil {
		return nil, nil
	}

	percent := *enrollment.Grades.CurrentScore

	courseName := course.Name
	if courseName == "" {
		courseName = "Unknown Course"
	}

	// Check if course uses weighted grading
	groups, err := r.client.AssignmentGroups(course.ID)
	if err != nil {
		groups = nil
	}

	weighted := isWeightedGrading(groups)

	if weighted {
		categories := r.buildCategoryGrades(course.ID, studentID, groups, current)
		return current, &CourseGrade{
			CourseName: courseName,
			Percent:    percent,
			Weighted:   true,
			Categories: categories,
		}
	}

	// Non-weighted: calculate points from submissions
	points := 0.0
	if enrollment.Grades.CurrentPoints != nil {
		points = *enrollment.Grades.CurrentPoints
	}
	pointsPossible := 0.0
	if percent > 0 {
		pointsPossible = points / (percent / 100)
	}

	return current, &CourseGrade{
		CourseName:     courseName,
		Points:         points,
		PointsPossible: pointsPossible,
		Percent:        percent,
		Weighted:       false,
	}
}

func isWeightedGrading(groups []AssignmentGroup) bool {
	for _, g := range groups {
		if g.GroupWeight > 0 {
			return true
		}
	}
	return false
}

func (r *Report) buildCategoryGrades(courseID, studentID int, groups []AssignmentGroup, period *GradingPeriod) []CategoryGrade {
	// Get submissions to calculate points per category
	submissions, err := r.client.Submissions(courseID, studentID)
	if err != nil {
		return nil
	}

	// Build map of assignment ID -> score
	scoreByAssignment := make(map[int]float64)
	for _, sub := range submissions {
		if sub.Score != nil {
			scoreByAssignment[sub.AssignmentID] = *sub.Score
		}
	}

	var categories []CategoryGrade
	for _, group := range groups {
		if group.GroupWeight == 0 {
			continue
		}

		var points, possible float64
		for _, a := range group.Assignments {
			if a.PointsPossible == nil || *a.PointsPossible == 0 {
				continue
			}
			if !assignmentInPeriod(a, period) {
				continue
			}
			// Only count assignments that have been graded
			if score, ok := scoreByAssignment[a.ID]; ok {
				points += score
				possible += *a.PointsPossible
			}
		}

		pct := 0.0
		if possible > 0 {
			pct = (points / possible) * 100
		}

		categories = append(categories, CategoryGrade{
			Name:           group.Name,
			Points:         points,
			PointsPossible: possible,
			Percent:        pct,
			Weight:         group.GroupWeight,
		})
	}

	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})

	return categories
}

type categoryState struct {
	points   float64
	possible float64
	weight   float64
}

type submissionInfo struct {
	score    *float64
	missing  bool
	graded   bool // Has GradedAt timestamp
}

func calculateAssignmentImpacts(
	groups []AssignmentGroup,
	submissions []Submission,
	currentOverall float64,
	weighted bool,
	period *GradingPeriod,
) map[int]*AssignmentImpact {
	impacts := make(map[int]*AssignmentImpact)

	// Build map of assignment submission info
	subInfoByAssignment := make(map[int]submissionInfo)
	for _, sub := range submissions {
		info := submissionInfo{
			missing: sub.Missing,
			graded:  sub.GradedAt != nil,
		}
		if sub.Score != nil {
			score := *sub.Score
			info.score = &score
		}
		subInfoByAssignment[sub.AssignmentID] = info
	}

	// Determine if an assignment is truly graded (score counts in totals)
	// Missing assignments with score=0 but no GradedAt are NOT in totals
	isGraded := func(id int) (bool, float64) {
		info, ok := subInfoByAssignment[id]
		if !ok || info.score == nil {
			return false, 0
		}
		// If marked missing and score is 0, only count if explicitly graded
		if info.missing && *info.score == 0 && !info.graded {
			return false, 0
		}
		return true, *info.score
	}

	// Build current state per category
	categoryStates := make(map[int]*categoryState)

	for _, group := range groups {
		state := &categoryState{weight: group.GroupWeight}
		for _, a := range group.Assignments {
			if a.PointsPossible == nil || *a.PointsPossible == 0 {
				continue
			}
			if !assignmentInPeriod(a, period) {
				continue
			}
			if graded, score := isGraded(a.ID); graded {
				state.points += score
				state.possible += *a.PointsPossible
			}
		}
		categoryStates[group.ID] = state
	}

	// For non-weighted courses, calculate total graded points
	var totalPoints, totalPossible float64
	if !weighted {
		for _, state := range categoryStates {
			totalPoints += state.points
			totalPossible += state.possible
		}
	}

	// Calculate impact for each assignment that could still improve
	for _, group := range groups {
		for _, a := range group.Assignments {
			if a.PointsPossible == nil || *a.PointsPossible == 0 {
				continue
			}
			if !assignmentInPeriod(a, period) {
				continue
			}

			pts := *a.PointsPossible
			state := categoryStates[group.ID]
			graded, score := isGraded(a.ID)

			// Skip if graded with non-zero score (no improvement possible in missing context)
			if graded && score > 0 {
				continue
			}

			if graded && score == 0 {
				// Graded as 0: already in totals, can only improve
				if weighted {
					impacts[a.ID] = calculateGradedZeroWeightedImpact(state, pts, currentOverall, categoryStates)
				} else {
					impacts[a.ID] = calculateGradedZeroNonWeightedImpact(totalPoints, totalPossible, pts, currentOverall)
				}
			} else {
				// Ungraded: not in totals yet
				if weighted {
					impacts[a.ID] = calculateWeightedImpact(state, pts, currentOverall, categoryStates)
				} else {
					impacts[a.ID] = calculateNonWeightedImpact(totalPoints, totalPossible, pts, currentOverall)
				}
			}
			impacts[a.ID].IsWeighted = weighted
		}
	}

	return impacts
}

func calculateWeightedImpact(
	category *categoryState,
	assignmentPts float64,
	currentOverall float64,
	allCategories map[int]*categoryState,
) *AssignmentImpact {
	// Zero-weight category has no impact
	if category.weight == 0 {
		return &AssignmentImpact{Gain: 0, Loss: 0}
	}

	// Helper to calculate overall grade with specified category values
	calcOverall := func(catPoints, catPossible float64, includeCategory bool) float64 {
		weightedSum := 0.0
		weightSum := 0.0
		for _, cat := range allCategories {
			if cat == category {
				if includeCategory && catPossible > 0 {
					catPct := (catPoints / catPossible) * 100
					weightedSum += catPct * cat.weight
					weightSum += cat.weight
				}
			} else if cat.possible > 0 {
				catPct := (cat.points / cat.possible) * 100
				weightedSum += catPct * cat.weight
				weightSum += cat.weight
			}
		}

		if weightSum == 0 {
			return 0
		}
		return weightedSum / weightSum
	}

	// Calculate current overall (without this assignment)
	// This ensures we compare apples-to-apples
	calcCurrentOverall := calcOverall(category.points, category.possible, category.possible > 0)

	// Best case: get 100% on assignment
	bestCatPts := category.points + assignmentPts
	bestCatPossible := category.possible + assignmentPts
	bestOverall := calcOverall(bestCatPts, bestCatPossible, true)

	// Worst case: get 0% on assignment
	worstCatPts := category.points
	worstCatPossible := category.possible + assignmentPts
	worstOverall := calcOverall(worstCatPts, worstCatPossible, true)

	return &AssignmentImpact{
		Gain: bestOverall - calcCurrentOverall,
		Loss: calcCurrentOverall - worstOverall,
	}
}

func calculateNonWeightedImpact(
	totalPoints, totalPossible float64,
	assignmentPts float64,
	currentOverall float64,
) *AssignmentImpact {
	// Calculate current percentage consistently
	var currentPct float64
	if totalPossible > 0 {
		currentPct = (totalPoints / totalPossible) * 100
	}

	// Best case: get 100% on assignment
	bestPts := totalPoints + assignmentPts
	bestPossible := totalPossible + assignmentPts
	var bestPct float64
	if bestPossible > 0 {
		bestPct = (bestPts / bestPossible) * 100
	}

	// Worst case: get 0% on assignment
	worstPts := totalPoints
	worstPossible := totalPossible + assignmentPts
	var worstPct float64
	if worstPossible > 0 {
		worstPct = (worstPts / worstPossible) * 100
	}

	return &AssignmentImpact{
		Gain: bestPct - currentPct,
		Loss: currentPct - worstPct,
	}
}

func calculateGradedZeroWeightedImpact(
	category *categoryState,
	assignmentPts float64,
	currentOverall float64,
	allCategories map[int]*categoryState,
) *AssignmentImpact {
	// Zero-weight category has no impact
	if category.weight == 0 {
		return &AssignmentImpact{Gain: 0, Loss: 0}
	}

	// For graded-0 assignments, the 0 and possible are already in the totals
	// Best case: improve from 0 to full points (add pts to earned, possible unchanged)
	// Worst case: stay at 0 (no change, loss = 0)

	// Helper to calculate overall grade with specified category values
	calcOverall := func(catPoints, catPossible float64) float64 {
		if catPossible == 0 {
			return 0
		}
		catPct := (catPoints / catPossible) * 100

		weightedSum := 0.0
		weightSum := 0.0
		for _, cat := range allCategories {
			if cat == category {
				weightedSum += catPct * cat.weight
				weightSum += cat.weight
			} else if cat.possible > 0 {
				otherPct := (cat.points / cat.possible) * 100
				weightedSum += otherPct * cat.weight
				weightSum += cat.weight
			}
		}

		if weightSum == 0 {
			return 0
		}
		return weightedSum / weightSum
	}

	// Calculate current overall (with the 0 score in place)
	calcCurrentOverall := calcOverall(category.points, category.possible)

	// Best case: improve from 0 to full points
	bestCatPts := category.points + assignmentPts // Add the points we'd gain
	bestCatPossible := category.possible          // Possible unchanged (already includes this assignment)
	bestOverall := calcOverall(bestCatPts, bestCatPossible)

	return &AssignmentImpact{
		Gain: bestOverall - calcCurrentOverall,
		Loss: 0, // Already at worst for this assignment
	}
}

func calculateGradedZeroNonWeightedImpact(
	totalPoints, totalPossible float64,
	assignmentPts float64,
	currentOverall float64,
) *AssignmentImpact {
	// For graded-0 assignments, the 0 and possible are already in the totals
	// Calculate current percentage consistently
	var currentPct float64
	if totalPossible > 0 {
		currentPct = (totalPoints / totalPossible) * 100
	}

	// Best case: improve from 0 to full points
	bestPts := totalPoints + assignmentPts
	bestPossible := totalPossible // Possible unchanged
	var bestPct float64
	if bestPossible > 0 {
		bestPct = (bestPts / bestPossible) * 100
	}

	return &AssignmentImpact{
		Gain: bestPct - currentPct,
		Loss: 0, // Already at worst for this assignment
	}
}

func assignmentInPeriod(a AssignmentInGroup, period *GradingPeriod) bool {
	if period == nil || period.StartDate == nil || period.EndDate == nil {
		return true // No period info, include everything
	}
	if a.DueAt == nil {
		return true // No due date, include to be safe
	}
	return !a.DueAt.Before(*period.StartDate) && !a.DueAt.After(*period.EndDate)
}

func currentGradingPeriod(periods []GradingPeriod) *GradingPeriod {
	now := time.Now()
	for i := range periods {
		p := &periods[i]
		if p.StartDate == nil || p.EndDate == nil {
			continue
		}
		if (now.Equal(*p.StartDate) || now.After(*p.StartDate)) &&
			(now.Equal(*p.EndDate) || now.Before(*p.EndDate)) {
			return p
		}
	}
	return nil
}

func (r *Report) missingAssignments(assignments []EnrichedAssignment) []EnrichedAssignment {
	now := time.Now()
	cutoff := now.Add(-oneMonthAgo)

	var result []EnrichedAssignment

	for _, a := range assignments {
		if a.DueAt.After(now) {
			continue
		}
		if !r.showAll && a.DueAt.Before(cutoff) {
			continue
		}

		sub := a.Submission
		isMissing := sub == nil || sub.Missing || (sub.Score != nil && *sub.Score == 0 && sub.GradedAt != nil)

		if isMissing && !awaitingGrade(sub) {
			enriched := a
			enriched.Status = determineStatus(a)
			result = append(result, enriched)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].DueAt.Before(result[j].DueAt)
	})

	return result
}

func (r *Report) upcomingAssignments(assignments []EnrichedAssignment) []EnrichedAssignment {
	now := time.Now()
	today := truncateToDay(now)
	tomorrow := nextSchoolDay(today)

	var result []EnrichedAssignment

	for _, a := range assignments {
		dueDate := truncateToDay(a.DueAt.Local())

		if dueDate.Equal(today) {
			if a.DueAt.After(now) {
				result = append(result, a)
			}
		} else if dueDate.Equal(tomorrow) {
			result = append(result, a)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].DueAt.Before(result[j].DueAt)
	})

	return result
}

func (r *Report) weekAheadAssignments(assignments []EnrichedAssignment) []EnrichedAssignment {
	today := truncateToDay(time.Now())
	tomorrow := nextSchoolDay(today)
	weekStart := tomorrow.AddDate(0, 0, 1)
	weekEnd := endOfSchoolWeek(today)

	if weekStart.After(weekEnd) {
		return nil
	}

	var result []EnrichedAssignment

	for _, a := range assignments {
		dueDate := truncateToDay(a.DueAt.Local())
		if (dueDate.Equal(weekStart) || dueDate.After(weekStart)) && (dueDate.Equal(weekEnd) || dueDate.Before(weekEnd)) {
			result = append(result, a)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].DueAt.Before(result[j].DueAt)
	})

	return result
}

func (r *Report) printReport(data studentData, colWidths columnWidths) {
	// Header box
	dateLine := "Generated: " + time.Now().Local().Format("Mon Jan 2, 2006 at 3:04 PM")
	width := len(data.name)
	if len(dateLine) > width {
		width = len(dateLine)
	}

	fmt.Println()
	fmt.Println("┌" + strings.Repeat("─", width+2) + "┐")
	fmt.Printf("│ %-*s │\n", width, data.name)
	fmt.Printf("│ %-*s │\n", width, dateLine)
	fmt.Println("└" + strings.Repeat("─", width+2) + "┘")
	fmt.Println()

	red := color.New(color.FgRed, color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow, color.Bold)
	cyan := color.New(color.FgCyan, color.Bold)
	dim := color.New(color.Faint)

	// Missing section
	if len(data.missing) == 0 {
		green.Printf("MISSING/INCOMPLETE (0)\n")
		color.New(color.FgGreen).Println("  All caught up!")
	} else {
		red.Printf("MISSING/INCOMPLETE (%d)\n", len(data.missing))
		r.printTable(data.missing, "missing", colWidths)
	}

	// Today/Tomorrow section
	fmt.Println()
	yellow.Printf("DUE TODAY/TOMORROW (%d pending)\n", data.upcomingPending)
	if len(data.upcoming) == 0 {
		dim.Println("  Nothing due today or tomorrow.")
	} else {
		r.printTable(data.upcoming, "upcoming", colWidths)
	}

	// Week Ahead section (only show if there are assignments)
	if len(data.weekAhead) > 0 {
		fmt.Println()
		cyan.Printf("WEEK AHEAD (%d pending)\n", data.weekAheadPending)
		r.printTable(data.weekAhead, "week_ahead", colWidths)
	}

	// Grades section
	r.printGrades(data.grades)

	// Summary
	fmt.Println()
	redText := color.New(color.FgRed)
	yellowText := color.New(color.FgYellow)
	cyanText := color.New(color.FgCyan)

	redText.Printf("%d missing", len(data.missing))
	fmt.Print(" | ")
	yellowText.Printf("%d due soon", data.upcomingPending)
	fmt.Print(" | ")
	cyanText.Printf("%d this week", data.weekAheadPending)
	fmt.Println()
}

func (r *Report) printTable(assignments []EnrichedAssignment, sectionType string, cw columnWidths) {
	widths := map[int]int{
		0: cw.subject,
		1: cw.assignment,
		2: cw.due,
		3: cw.pts,
		4: cw.impact,
		5: cw.status,
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Configure(func(cfg *tablewriter.Config) {
		cfg.Row.Formatting.AutoWrap = tw.WrapTruncate
		cfg.Row.Alignment.PerColumn = []tw.Align{
			tw.AlignLeft,  // Subject
			tw.AlignLeft,  // Assignment
			tw.AlignLeft,  // Due
			tw.AlignRight, // Pts
			tw.AlignRight, // Impact
			tw.AlignLeft,  // Status
		}
		cfg.Widths.PerColumn = widths
	})
	table.Header("Subject", "Assignment", "Due", "Pts", "Impact", "")

	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)
	dim := color.New(color.Faint)

	for _, a := range assignments {
		subject := truncateString(a.CourseName, cw.subject)
		name := formatAssignmentName(a.Name, a.CategoryName, cw.assignment, dim)
		due := strings.ToLower(a.DueAt.Local().Format("Mon 1/2 3pm"))
		pts := ""
		if a.PointsPossible != nil {
			pts = fmt.Sprintf("%d", int(*a.PointsPossible))
		}

		if sectionType == "missing" {
			impact := formatImpact(a.Impact)
			var status string
			if a.Status == "Missing" {
				status = red.Sprint("✗")
			} else {
				status = red.Sprint("0")
			}
			table.Append(subject, name, due, pts, impact, status)
		} else if isCompleted(a.Submission) {
			// Don't show impact for completed assignments
			table.Append(
				dim.Sprint(subject),
				dim.Sprint(name),
				dim.Sprint(due),
				dim.Sprint(pts),
				"",
				green.Sprint("✓"),
			)
		} else {
			impact := formatImpact(a.Impact)
			table.Append(subject, name, due, pts, impact, "")
		}
	}

	table.Render()
}

func formatImpact(impact *AssignmentImpact) string {
	if impact == nil {
		return "-"
	}

	// Clamp to non-negative values (negative shouldn't occur in valid scenarios)
	gain := impact.Gain
	if gain < 0 {
		gain = 0
	}
	loss := impact.Loss
	if loss < 0 {
		loss = 0
	}

	// Round to 1 decimal for display comparison
	gainRounded := float64(int(gain*10+0.5)) / 10
	lossRounded := float64(int(loss*10+0.5)) / 10

	hasGain := gainRounded >= 0.1
	hasLoss := lossRounded >= 0.1

	if !hasGain && !hasLoss {
		return "-"
	}
	if hasGain && !hasLoss {
		return fmt.Sprintf("+%.1f%%", gain)
	}
	if !hasGain && hasLoss {
		return fmt.Sprintf("-%.1f%%", loss)
	}
	return fmt.Sprintf("+%.1f/-%.1f%%", gain, loss)
}

func formatAssignmentName(name, category string, maxWidth int, dim *color.Color) string {
	if category == "" {
		return truncateString(name, maxWidth)
	}

	// Combine name and category, then truncate wherever it lands
	suffix := " (" + category + ")"
	full := name + suffix

	if len(full) <= maxWidth {
		return name + dim.Sprint(suffix)
	}

	// Truncate the combined string
	truncated := truncateString(full, maxWidth)

	// Find where the dim part should start (if suffix is still partially visible)
	if len(truncated) > len(name) {
		return name + dim.Sprint(truncated[len(name):])
	}
	return truncated
}

func (r *Report) printGrades(grades []periodGrades) {
	if len(grades) == 0 {
		return
	}

	magenta := color.New(color.FgMagenta, color.Bold)
	dim := color.New(color.Faint)

	for _, pg := range grades {
		// Format period header
		periodName := pg.period.Title
		if periodName == "" {
			periodName = "Current Period"
		}
		dateRange := ""
		if pg.period.StartDate != nil && pg.period.EndDate != nil {
			dateRange = fmt.Sprintf(" (%s - %s)",
				pg.period.StartDate.Local().Format("Jan 2"),
				pg.period.EndDate.Local().Format("Jan 2"))
		}

		fmt.Println()
		magenta.Printf("GRADES - %s%s\n", periodName, dateRange)

		table := tablewriter.NewWriter(os.Stdout)
		table.Configure(func(cfg *tablewriter.Config) {
			cfg.Row.Formatting.AutoWrap = tw.WrapTruncate
			cfg.Row.Alignment.PerColumn = []tw.Align{
				tw.AlignLeft,  // Subject
				tw.AlignRight, // %
				tw.AlignRight, // Points
				tw.AlignRight, // Possible
				tw.AlignRight, // Weight
			}
		})
		table.Header("Subject", "%", "Points", "Possible", "Weight")

		for _, g := range pg.grades {
			if g.Weighted {
				// Weighted course: summary row, then indented categories
				table.Append(
					g.CourseName,
					fmt.Sprintf("%.2f%%", g.Percent),
					"",
					"",
					"",
				)
				for _, cat := range g.Categories {
					table.Append(
						dim.Sprintf("  %s", cat.Name),
						dim.Sprintf("%.2f%%", cat.Percent),
						dim.Sprintf("%.0f", cat.Points),
						dim.Sprintf("%.0f", cat.PointsPossible),
						dim.Sprintf("%.0f%%", cat.Weight),
					)
				}
			} else {
				// Non-weighted course: simple row
				table.Append(
					g.CourseName,
					fmt.Sprintf("%.2f%%", g.Percent),
					fmt.Sprintf("%.0f", g.Points),
					fmt.Sprintf("%.0f", g.PointsPossible),
					"",
				)
			}
		}

		table.Render()
	}
}

func isCompleted(sub *Submission) bool {
	if sub == nil {
		return false
	}
	return sub.SubmittedAt != nil || sub.GradedAt != nil || sub.Excused
}

func awaitingGrade(sub *Submission) bool {
	if sub == nil {
		return false
	}
	if sub.SubmittedAt == nil {
		return false
	}
	return sub.GradeMatchesCurrentSubmission != nil && !*sub.GradeMatchesCurrentSubmission
}

func determineStatus(a EnrichedAssignment) string {
	sub := a.Submission
	if sub != nil && sub.Score != nil && *sub.Score == 0 && sub.GradedAt != nil {
		if a.PointsPossible != nil {
			return fmt.Sprintf("Graded 0/%d", int(*a.PointsPossible))
		}
		return "Graded 0"
	}
	return "Missing"
}

func countPending(assignments []EnrichedAssignment) int {
	count := 0
	for _, a := range assignments {
		if !isCompleted(a.Submission) {
			count++
		}
	}
	return count
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func nextSchoolDay(date time.Time) time.Time {
	switch date.Weekday() {
	case time.Friday:
		return date.AddDate(0, 0, 3) // Monday
	case time.Saturday:
		return date.AddDate(0, 0, 2) // Monday
	default:
		return date.AddDate(0, 0, 1)
	}
}

func endOfSchoolWeek(date time.Time) time.Time {
	switch date.Weekday() {
	case time.Sunday:
		return date.AddDate(0, 0, 5) // Friday
	case time.Saturday:
		return date.AddDate(0, 0, 6) // Next Friday
	case time.Friday:
		return date.AddDate(0, 0, 7) // Next Friday
	default:
		daysUntilFriday := 5 - int(date.Weekday())
		return date.AddDate(0, 0, daysUntilFriday)
	}
}
