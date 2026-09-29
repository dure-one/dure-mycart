package models

import (
	"testing"
)

func TestMessageCreateValidation(t *testing.T) {
	tests := []struct {
		name    string
		msg     MessageCreate
		wantErr bool
	}{
		{
			name: "valid sms",
			msg: MessageCreate{
				ContactAddress: "+12125551234",
				ContactType:    "sms",
				Content:        "Hello",
				Direction:      "inbound",
				Channel:        "sms",
			},
			wantErr: false,
		},
		{
			name: "valid xmpp",
			msg: MessageCreate{
				ContactAddress: "user@example.com",
				ContactType:    "xmpp",
				Content:        "Hello",
				Direction:      "inbound",
				Channel:        "xmpp",
			},
			wantErr: false,
		},
		{
			name: "missing content",
			msg: MessageCreate{
				ContactAddress: "+12125551234",
				ContactType:    "sms",
				Direction:      "inbound",
				Channel:        "sms",
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			msg: MessageCreate{
				ContactAddress: "+12125551234",
				ContactType:    "telegram",
				Content:        "Hello",
				Direction:      "inbound",
				Channel:        "sms",
			},
			wantErr: true,
		},
		{
			name: "content too long",
			msg: MessageCreate{
				ContactAddress: "+12125551234",
				ContactType:    "sms",
				Content:        string(make([]byte, 10001)),
				Direction:      "inbound",
				Channel:        "sms",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWorkflowValidation(t *testing.T) {
	tests := []struct {
		name     string
		workflow Workflow
		wantErr  bool
	}{
		{
			name: "valid workflow",
			workflow: Workflow{
				Name:    "Order Processing",
				Content: "graph TD\nA --> B",
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name: "name too short",
			workflow: Workflow{
				Name:    "",
				Content: "graph TD\nA --> B",
			},
			wantErr: true,
		},
		{
			name: "name too long",
			workflow: Workflow{
				Name:    string(make([]byte, 201)),
				Content: "graph TD\nA --> B",
			},
			wantErr: true,
		},
		{
			name: "missing content",
			workflow: Workflow{
				Name: "Test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.workflow.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
