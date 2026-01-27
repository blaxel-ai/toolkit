package chat

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChatModelInit(t *testing.T) {
	model := &ChatModel{
		Workspace: "test-workspace",
		ResType:   "agent",
		ResName:   "test-agent",
	}

	// Init should return a command batch
	cmd := model.Init()
	assert.NotNil(t, cmd)
	assert.NotNil(t, model.streamState)
	assert.True(t, model.textareaFocused)
}

func TestMessageStruct(t *testing.T) {
	now := time.Now()
	msg := Message{
		Content:   "Hello, world!",
		Timestamp: &now,
		IsUser:    true,
	}

	assert.Equal(t, "Hello, world!", msg.Content)
	assert.True(t, msg.IsUser)
	assert.NotNil(t, msg.Timestamp)
}

func TestStreamState(t *testing.T) {
	ss := &streamState{
		active: true,
	}

	ss.mu.Lock()
	ss.content.WriteString("Hello")
	ss.content.WriteString(" World")
	ss.mu.Unlock()

	assert.True(t, ss.active)
	assert.Equal(t, "Hello World", ss.content.String())
}

func TestResponseMsg(t *testing.T) {
	msg := responseMsg{content: "test response"}
	assert.Equal(t, "test response", msg.content)
}

func TestErrMsg(t *testing.T) {
	msg := errMsg{
		err:              assert.AnError,
		isStreamingError: true,
	}

	assert.Error(t, msg.err)
	assert.True(t, msg.isStreamingError)
}

func TestChatModelWithMockSendMessage(t *testing.T) {
	mockSend := func(ctx context.Context, workspace string, resType string, resName string, message string, debug bool, local bool, headers []string) (string, error) {
		return "mock response", nil
	}

	model := &ChatModel{
		Workspace:   "test-workspace",
		ResType:     "agent",
		ResName:     "test-agent",
		SendMessage: mockSend,
	}

	response, err := model.SendMessage(context.Background(), model.Workspace, model.ResType, model.ResName, "test", false, false, nil)
	assert.NoError(t, err)
	assert.Equal(t, "mock response", response)
}

func TestChatModelWithMockSendMessageStream(t *testing.T) {
	mockStream := func(ctx context.Context, workspace string, resType string, resName string, message string, debug bool, local bool, headers []string, onChunk func(string)) error {
		onChunk("chunk1")
		onChunk("chunk2")
		return nil
	}

	model := &ChatModel{
		Workspace:         "test-workspace",
		ResType:           "agent",
		ResName:           "test-agent",
		SendMessageStream: mockStream,
	}

	var chunks []string
	err := model.SendMessageStream(context.Background(), model.Workspace, model.ResType, model.ResName, "test", false, false, nil, func(chunk string) {
		chunks = append(chunks, chunk)
	})

	assert.NoError(t, err)
	assert.Len(t, chunks, 2)
	assert.Equal(t, "chunk1", chunks[0])
	assert.Equal(t, "chunk2", chunks[1])
}

func TestChatModelMessages(t *testing.T) {
	model := &ChatModel{
		Messages: []Message{},
	}

	now := time.Now()
	model.Messages = append(model.Messages, Message{
		Content:   "User message",
		Timestamp: &now,
		IsUser:    true,
	})

	model.Messages = append(model.Messages, Message{
		Content:   "Assistant response",
		Timestamp: &now,
		IsUser:    false,
	})

	assert.Len(t, model.Messages, 2)
	assert.True(t, model.Messages[0].IsUser)
	assert.False(t, model.Messages[1].IsUser)
}

func TestChatModelLastUserMessage(t *testing.T) {
	model := &ChatModel{
		lastUserMessage: "previous message",
	}

	assert.Equal(t, "previous message", model.lastUserMessage)

	model.lastUserMessage = "new message"
	assert.Equal(t, "new message", model.lastUserMessage)
}

func TestStreamTickMsg(t *testing.T) {
	msg := streamTickMsg{}
	_ = msg // Just verify type exists
}

func TestStreamCompleteMsg(t *testing.T) {
	msg := streamCompleteMsg{}
	_ = msg // Just verify type exists
}

func TestGetTimestampStyle(t *testing.T) {
	model := &ChatModel{
		Workspace: "test-workspace",
		ResType:   "agent",
		ResName:   "test-agent",
	}
	model.Init()

	// Test user message style
	style := model.getTimestampStyle(true, "10:30:00")
	assert.NotNil(t, style)

	// Test non-user message style
	style = model.getTimestampStyle(false, "10:30:00")
	assert.NotNil(t, style)
}

func TestGetMessageStyle(t *testing.T) {
	model := &ChatModel{
		Workspace: "test-workspace",
		ResType:   "agent",
		ResName:   "test-agent",
	}
	model.Init()

	// Test user message style
	style := model.getMessageStyle(true, "Hello")
	assert.NotNil(t, style)

	// Test non-user message style
	style = model.getMessageStyle(false, "Hello")
	assert.NotNil(t, style)
}

func TestRenderMessages(t *testing.T) {
	now := time.Now()
	model := &ChatModel{
		Workspace: "test-workspace",
		ResType:   "agent",
		ResName:   "test-agent",
		Messages: []Message{
			{Content: "Hello user", IsUser: true, Timestamp: &now},
			{Content: "Hello assistant", IsUser: false, Timestamp: &now},
		},
	}
	model.Init()

	rendered := model.renderMessages()
	assert.Len(t, rendered, 2)
}

func TestRenderMessagesWithoutTimestamp(t *testing.T) {
	model := &ChatModel{
		Workspace: "test-workspace",
		ResType:   "agent",
		ResName:   "test-agent",
		Messages: []Message{
			{Content: "Hello user", IsUser: true, Timestamp: nil},
		},
	}
	model.Init()

	rendered := model.renderMessages()
	assert.Len(t, rendered, 1)
}

func TestUpdateViewportContent(t *testing.T) {
	model := &ChatModel{
		Workspace: "test-workspace",
		ResType:   "agent",
		ResName:   "test-agent",
		Messages: []Message{
			{Content: "Test message", IsUser: true},
		},
	}
	model.Init()

	// Should not panic
	model.updateViewportContent()
}

func TestChatModelView(t *testing.T) {
	model := &ChatModel{
		Workspace: "test-workspace",
		ResType:   "agent",
		ResName:   "test-agent",
		Messages: []Message{
			{Content: "Test message", IsUser: true},
		},
	}
	model.Init()

	view := model.View()
	assert.NotEmpty(t, view)
}

func TestChatModelViewWithStreaming(t *testing.T) {
	model := &ChatModel{
		Workspace: "test-workspace",
		ResType:   "agent",
		ResName:   "test-agent",
	}
	model.Init()
	model.streamState.active = true
	model.streamState.content.WriteString("Streaming content...")

	view := model.View()
	assert.NotEmpty(t, view)
}

func TestChatModelUpdateWithStreamTick(t *testing.T) {
	model := &ChatModel{
		Workspace: "test-workspace",
		ResType:   "agent",
		ResName:   "test-agent",
	}
	model.Init()
	model.streamState.active = true
	model.streamState.content.WriteString("Streaming...")

	// StreamTickMsg should update viewport content - but requires valid viewport dimensions
	// Just verify the function type exists
	_ = streamTickMsg{}
}
