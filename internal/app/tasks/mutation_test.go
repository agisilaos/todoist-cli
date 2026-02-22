package tasks

import (
	"errors"
	"testing"
)

type stubSelectorResolver struct {
	projectID string
	sectionID string
	assignee  string
	err       error
}

func (s stubSelectorResolver) ResolveProjectSelector(explicitID, reference string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if explicitID != "" {
		return explicitID, nil
	}
	return s.projectID, nil
}

func (s stubSelectorResolver) ResolveSectionSelector(explicitID, reference, projectRef string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if explicitID != "" {
		return explicitID, nil
	}
	return s.sectionID, nil
}

func (s stubSelectorResolver) ResolveAssigneeSelector(explicitID, reference, projectRef, taskID string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if explicitID != "" {
		return explicitID, nil
	}
	return s.assignee, nil
}

func TestBuildCreatePayload(t *testing.T) {
	body, err := BuildCreatePayload(MutationInput{
		Content:   "Write docs",
		ProjectID: "p1",
		SectionID: "s1",
		Priority:  4,
	}, stubSelectorResolver{})
	if err != nil {
		t.Fatalf("BuildCreatePayload: %v", err)
	}
	if body["content"] != "Write docs" || body["project_id"] != "p1" || body["section_id"] != "s1" || body["priority"] != 4 {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestBuildMovePayload(t *testing.T) {
	body, err := BuildMovePayload("p2", "", "s2", "", "t0", stubSelectorResolver{})
	if err != nil {
		t.Fatalf("BuildMovePayload: %v", err)
	}
	if body["project_id"] != "p2" || body["section_id"] != "s2" || body["parent_id"] != "t0" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestBuildUpdatePayloadResolverError(t *testing.T) {
	_, err := BuildUpdatePayload(MutationInput{}, stubSelectorResolver{err: errors.New("boom")})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom error, got %v", err)
	}
}
