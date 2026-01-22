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
	status     int
}

func calculateColumnWidths(assignments ...[]EnrichedAssignment) columnWidths {
	// Fixed widths for predictable columns
	const (
		dueWidth    = 18 // "thu 12/18 11pm" + padding
		ptsWidth    = 5
		statusWidth = 3
		// Table overhead: 5 column separators (│) + padding (2 per col) = ~17 chars
		overhead = 17
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
	flexible := termWidth - dueWidth - ptsWidth - statusWidth - overhead

	// If everything fits, use actual widths
	if maxSubject+maxAssignment <= flexible {
		return columnWidths{
			subject:    maxSubject,
			assignment: maxAssignment,
			due:        dueWidth,
			pts:        ptsWidth,
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
		status:     statusWidth,
	}
}

type EnrichedAssignment struct {
	Name           string
	CourseName     string
	DueAt          time.Time
	PointsPossible *float64
	Submission     *Submission
	Status         string
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
	tableWidth := colWidths.subject + colWidths.assignment + colWidths.due + colWidths.pts + colWidths.status + 16

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
			DueAt:          *a.DueAt,
			PointsPossible: a.PointsPossible,
			Submission:     submissionsByID[a.ID],
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
	points := 0.0
	if enrollment.Grades.CurrentPoints != nil {
		points = *enrollment.Grades.CurrentPoints
	}

	// Calculate points possible from points and percent
	pointsPossible := 0.0
	if percent > 0 {
		pointsPossible = points / (percent / 100)
	}

	courseName := course.Name
	if courseName == "" {
		courseName = "Unknown Course"
	}

	return current, &CourseGrade{
		CourseName:     courseName,
		Points:         points,
		PointsPossible: pointsPossible,
		Percent:        percent,
	}
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
		4: cw.status,
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Configure(func(cfg *tablewriter.Config) {
		cfg.Row.Formatting.AutoWrap = tw.WrapTruncate
		cfg.Row.Alignment.PerColumn = []tw.Align{
			tw.AlignLeft,  // Subject
			tw.AlignLeft,  // Assignment
			tw.AlignLeft,  // Due
			tw.AlignRight, // Pts
			tw.AlignLeft,  // Status
		}
		cfg.Widths.PerColumn = widths
	})
	table.Header("Subject", "Assignment", "Due", "Pts", "")

	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)
	dim := color.New(color.Faint)

	for _, a := range assignments {
		subject := truncateString(a.CourseName, cw.subject)
		name := truncateString(a.Name, cw.assignment)
		due := strings.ToLower(a.DueAt.Local().Format("Mon 1/2 3pm"))
		pts := ""
		if a.PointsPossible != nil {
			pts = fmt.Sprintf("%d", int(*a.PointsPossible))
		}

		if sectionType == "missing" {
			var status string
			if a.Status == "Missing" {
				status = red.Sprint("✗")
			} else {
				status = red.Sprint("0")
			}
			table.Append(subject, name, due, pts, status)
		} else if isCompleted(a.Submission) {
			table.Append(
				dim.Sprint(subject),
				dim.Sprint(name),
				dim.Sprint(due),
				dim.Sprint(pts),
				green.Sprint("✓"),
			)
		} else {
			table.Append(subject, name, due, pts, "")
		}
	}

	table.Render()
}

func (r *Report) printGrades(grades []periodGrades) {
	if len(grades) == 0 {
		return
	}

	magenta := color.New(color.FgMagenta, color.Bold)

	for _, pg := range grades {
		fmt.Println()

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
		magenta.Printf("GRADES - %s%s\n", periodName, dateRange)

		table := tablewriter.NewWriter(os.Stdout)
		table.Configure(func(cfg *tablewriter.Config) {
			cfg.Row.Formatting.AutoWrap = tw.WrapTruncate
			cfg.Row.Alignment.PerColumn = []tw.Align{
				tw.AlignLeft,  // Subject
				tw.AlignRight, // Points
				tw.AlignRight, // Possible
				tw.AlignRight, // %
			}
		})
		table.Header("Subject", "Points", "Possible", "%")

		for _, g := range pg.grades {
			table.Append(
				g.CourseName,
				fmt.Sprintf("%.0f", g.Points),
				fmt.Sprintf("%.0f", g.PointsPossible),
				fmt.Sprintf("%.2f%%", g.Percent),
			)
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
