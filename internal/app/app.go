package app

import (
	appagent "github.com/agisilaos/todoist-cli/internal/app/agent"
	apptasks "github.com/agisilaos/todoist-cli/internal/app/tasks"
)

type Services struct {
	Tasks apptasks.Service
	Agent appagent.Service
}

func NewServices() Services {
	return Services{
		Tasks: apptasks.Service{},
		Agent: appagent.Service{},
	}
}
