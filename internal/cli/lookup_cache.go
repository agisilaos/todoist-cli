package cli

import "github.com/agisilaos/todoist-cli/internal/api"

type lookupCache struct {
	projectsLoaded bool
	projects       []api.Project

	sectionsByProject map[string][]api.Section

	labelsLoaded bool
	labels       []api.Label

	activeTasksLoaded bool
	activeTasks       []api.Task

	filtersLoaded bool
	filters       []api.Filter

	collaboratorsByProject map[string][]api.Collaborator

	workspacesLoaded bool
	workspaces       []api.Workspace
}

func (ctx *Context) cache() *lookupCache {
	if ctx == nil {
		return nil
	}
	if ctx.lookupCache == nil {
		ctx.lookupCache = &lookupCache{
			sectionsByProject:      map[string][]api.Section{},
			collaboratorsByProject: map[string][]api.Collaborator{},
		}
	}
	return ctx.lookupCache
}

func cloneSlice[T any](in []T) []T {
	if len(in) == 0 {
		return nil
	}
	out := make([]T, len(in))
	copy(out, in)
	return out
}
