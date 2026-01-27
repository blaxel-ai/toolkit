package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusConstants(t *testing.T) {
	assert.Equal(t, Status(0), StatusPending)
	assert.Equal(t, Status(1), StatusRunning)
	assert.Equal(t, Status(2), StatusComplete)
	assert.Equal(t, Status(3), StatusFailed)
}

func TestStepStruct(t *testing.T) {
	step := Step{
		Title:       "Test Step",
		Description: "Test Description",
		Status:      StatusPending,
	}

	assert.Equal(t, "Test Step", step.Title)
	assert.Equal(t, "Test Description", step.Description)
	assert.Equal(t, StatusPending, step.Status)
}

func TestNewInstallationModel(t *testing.T) {
	model := NewInstallationModel()

	assert.Len(t, model.steps, 4)
	assert.Equal(t, -1, model.currentStep)
	assert.False(t, model.done)
	assert.Nil(t, model.err)
	assert.False(t, model.showLogs)
	assert.Equal(t, 4, model.maxLogLines)
	assert.Equal(t, 0, model.logOffset)
}

func TestModelStartStep(t *testing.T) {
	model := NewInstallationModel()

	model.StartStep(0)

	assert.Equal(t, 0, model.currentStep)
	assert.Equal(t, StatusRunning, model.steps[0].Status)
}

func TestModelStartStepOutOfBounds(t *testing.T) {
	model := NewInstallationModel()

	// Should not panic
	model.StartStep(-1)
	model.StartStep(100)
}

func TestModelCompleteStep(t *testing.T) {
	model := NewInstallationModel()
	model.StartStep(0)

	model.CompleteStep(0)

	assert.Equal(t, StatusComplete, model.steps[0].Status)
}

func TestModelCompleteStepOutOfBounds(t *testing.T) {
	model := NewInstallationModel()

	// Should not panic
	model.CompleteStep(-1)
	model.CompleteStep(100)
}

func TestModelFailStep(t *testing.T) {
	model := NewInstallationModel()
	model.StartStep(0)

	err := assert.AnError
	model.FailStep(0, err)

	assert.Equal(t, StatusFailed, model.steps[0].Status)
	assert.Equal(t, err, model.err)
}

func TestModelFailStepOutOfBounds(t *testing.T) {
	model := NewInstallationModel()

	// Should not panic
	model.FailStep(-1, assert.AnError)
	model.FailStep(100, assert.AnError)
}

func TestModelDone(t *testing.T) {
	model := NewInstallationModel()

	model.Done()

	assert.True(t, model.done)
}

func TestModelUpdateVisibleLogs(t *testing.T) {
	model := NewInstallationModel()
	model.logBuffer = []string{"line1", "line2", "line3", "line4", "line5"}
	model.logOffset = 0
	model.maxLogLines = 3

	model.updateVisibleLogs()

	assert.Len(t, model.currentLogs, 3)
	assert.Equal(t, "line1", model.currentLogs[0])
	assert.Equal(t, "line2", model.currentLogs[1])
	assert.Equal(t, "line3", model.currentLogs[2])
}

func TestModelUpdateVisibleLogsWithOffset(t *testing.T) {
	model := NewInstallationModel()
	model.logBuffer = []string{"line1", "line2", "line3", "line4", "line5"}
	model.logOffset = 2
	model.maxLogLines = 3

	model.updateVisibleLogs()

	assert.Len(t, model.currentLogs, 3)
	assert.Equal(t, "line3", model.currentLogs[0])
	assert.Equal(t, "line4", model.currentLogs[1])
	assert.Equal(t, "line5", model.currentLogs[2])
}

func TestModelView(t *testing.T) {
	model := NewInstallationModel()

	view := model.View()

	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Installing")
	assert.Contains(t, view, "Creating project")
	assert.Contains(t, view, "Cloning repository")
	assert.Contains(t, view, "Installing dependencies")
	assert.Contains(t, view, "Finalizing")
}

func TestModelViewWhenDone(t *testing.T) {
	model := NewInstallationModel()
	model.done = true

	view := model.View()

	assert.Empty(t, view)
}

func TestGetDependencyManager(t *testing.T) {
	// Test with unknown language
	manager, version := GetDependencyManager("unknown", "/tmp")
	assert.Empty(t, manager)
	assert.Empty(t, version)
}

func TestGetDependencyManagerPython(t *testing.T) {
	// Test python dependency manager detection
	// Results depend on installed tools
	manager, version := GetDependencyManager("python", "/tmp")

	// If any python package manager is available, we should get a result
	if manager != "" {
		assert.True(t, manager == "uv" || manager == "pip")
		assert.NotEmpty(t, version)
	}
}

func TestGetDependencyManagerTypescript(t *testing.T) {
	// Test typescript dependency manager detection
	// Results depend on installed tools
	manager, version := GetDependencyManager("typescript", "/tmp")

	// If any typescript package manager is available, we should get a result
	if manager != "" {
		assert.True(t, manager == "pnpm" || manager == "npm")
		assert.NotEmpty(t, version)
	}
}

func TestStepMsgStruct(t *testing.T) {
	msg := stepMsg{
		step:   1,
		status: StatusRunning,
	}

	assert.Equal(t, 1, msg.step)
	assert.Equal(t, StatusRunning, msg.status)
}

func TestUpdateDescriptionMsgStruct(t *testing.T) {
	msg := updateDescriptionMsg{
		step: 2,
		desc: "Installing with npm",
	}

	assert.Equal(t, 2, msg.step)
	assert.Equal(t, "Installing with npm", msg.desc)
}

func TestEnableLogsMsgType(t *testing.T) {
	msg := enableLogsMsg(true)
	assert.True(t, bool(msg))

	msg = enableLogsMsg(false)
	assert.False(t, bool(msg))
}
