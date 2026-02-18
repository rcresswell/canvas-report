// ABOUTME: HTTP client for the Canvas LMS API.
// ABOUTME: Handles authentication, pagination, and fetching observees, courses, assignments, and submissions.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type CanvasClient struct {
	baseURL     string
	accessToken string
	httpClient  *http.Client
}

type Observee struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
}

type Course struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Assignment struct {
	ID             int       `json:"id"`
	Name           string    `json:"name"`
	DueAt          *time.Time `json:"due_at"`
	PointsPossible *float64  `json:"points_possible"`
}

type Submission struct {
	AssignmentID                  int        `json:"assignment_id"`
	SubmittedAt                   *time.Time `json:"submitted_at"`
	GradedAt                      *time.Time `json:"graded_at"`
	Score                         *float64   `json:"score"`
	Missing                       bool       `json:"missing"`
	Excused                       bool       `json:"excused"`
	GradeMatchesCurrentSubmission *bool      `json:"grade_matches_current_submission"`
}

type GradingPeriod struct {
	ID        any        `json:"id"`
	Title     string     `json:"title"`
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
}

type Enrollment struct {
	Grades struct {
		CurrentScore  *float64 `json:"current_score"`
		CurrentPoints *float64 `json:"current_points"`
	} `json:"grades"`
}

type AssignmentGroup struct {
	ID          int                   `json:"id"`
	Name        string                `json:"name"`
	GroupWeight float64               `json:"group_weight"`
	Assignments []AssignmentInGroup   `json:"assignments"`
}

type AssignmentInGroup struct {
	ID             int        `json:"id"`
	Name           string     `json:"name"`
	PointsPossible *float64   `json:"points_possible"`
	DueAt          *time.Time `json:"due_at"`
}

func NewCanvasClient(baseURL, accessToken string) *CanvasClient {
	return &CanvasClient{
		baseURL:     strings.TrimSuffix(baseURL, "/"),
		accessToken: accessToken,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *CanvasClient) Observees() ([]Observee, error) {
	return getPaginated[Observee](c, "/api/v1/users/self/observees", nil)
}

func (c *CanvasClient) Courses(userID int) ([]Course, error) {
	params := url.Values{"enrollment_state": []string{"active"}}
	return getPaginated[Course](c, fmt.Sprintf("/api/v1/users/%d/courses", userID), params)
}

func (c *CanvasClient) Assignments(courseID int) ([]Assignment, error) {
	params := url.Values{"per_page": []string{"100"}}
	return getPaginated[Assignment](c, fmt.Sprintf("/api/v1/courses/%d/assignments", courseID), params)
}

func (c *CanvasClient) Submissions(courseID, studentID int) ([]Submission, error) {
	params := url.Values{
		"student_ids[]": []string{fmt.Sprintf("%d", studentID)},
		"per_page":      []string{"100"},
	}
	return getPaginated[Submission](c, fmt.Sprintf("/api/v1/courses/%d/students/submissions", courseID), params)
}

type gradingPeriodsResponse struct {
	GradingPeriods []GradingPeriod `json:"grading_periods"`
}

func (c *CanvasClient) GradingPeriods(courseID int) ([]GradingPeriod, error) {
	fullURL := fmt.Sprintf("%s/api/v1/courses/%d/grading_periods", c.baseURL, courseID)

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Canvas API error: %d - %s", resp.StatusCode, string(body))
	}

	var result gradingPeriodsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.GradingPeriods, nil
}

func (c *CanvasClient) Enrollments(courseID, studentID int, gradingPeriodID string) ([]Enrollment, error) {
	params := url.Values{
		"user_id":   []string{fmt.Sprintf("%d", studentID)},
		"type[]":    []string{"StudentEnrollment"},
		"include[]": []string{"current_points"},
	}
	if gradingPeriodID != "" {
		params.Set("grading_period_id", gradingPeriodID)
	}
	return getPaginated[Enrollment](c, fmt.Sprintf("/api/v1/courses/%d/enrollments", courseID), params)
}

func (c *CanvasClient) AssignmentGroups(courseID int) ([]AssignmentGroup, error) {
	params := url.Values{
		"include[]": []string{"assignments"},
	}
	return getPaginated[AssignmentGroup](c, fmt.Sprintf("/api/v1/courses/%d/assignment_groups", courseID), params)
}

func getPaginated[T any](c *CanvasClient, path string, params url.Values) ([]T, error) {
	var result []T

	fullURL := c.baseURL + path
	if params != nil {
		fullURL += "?" + params.Encode()
	}

	for fullURL != "" {
		req, err := http.NewRequest("GET", fullURL, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+c.accessToken)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("Canvas API error: %d - %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		var page []T
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, err
		}

		result = append(result, page...)

		fullURL = parseNextLink(resp.Header.Get("Link"))
	}

	return result, nil
}

var linkNextRegex = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

func parseNextLink(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	matches := linkNextRegex.FindStringSubmatch(linkHeader)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}
