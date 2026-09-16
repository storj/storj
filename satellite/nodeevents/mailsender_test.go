// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeevents_test

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/post"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/nodeevents"
)

var errMailFailure = errors.New("smtp unavailable")

// captureSender records messages without rendering them.
type captureSender struct {
	mu   sync.Mutex
	sent []mailservice.Message
	err  error
}

func (s *captureSender) SendRendered(_ context.Context, _ []post.Address, msg mailservice.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, msg)
	return nil
}

func (s *captureSender) messages() []mailservice.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]mailservice.Message{}, s.sent...)
}

// capturePostSender records fully rendered messages.
type capturePostSender struct {
	mu   sync.Mutex
	sent []*post.Message
}

func (s *capturePostSender) SendEmail(_ context.Context, msg *post.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, msg)
	return nil
}

func (s *capturePostSender) FromAddress() post.Address {
	return post.Address{Address: "storj@mail.test"}
}

func (s *capturePostSender) last() *post.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.sent) == 0 {
		return nil
	}
	return s.sent[len(s.sent)-1]
}

var allEventTypes = []nodeevents.Type{
	nodeevents.Online,
	nodeevents.Offline,
	nodeevents.Disqualified,
	nodeevents.UnknownAuditSuspended,
	nodeevents.UnknownAuditUnsuspended,
	nodeevents.OfflineSuspended,
	nodeevents.OfflineUnsuspended,
	nodeevents.BelowMinVersion,
}

func TestMailNotifier(t *testing.T) {
	ctx := testcontext.New(t)
	node1, node2 := testrand.NodeID(), testrand.NodeID()
	ipPort := "1.2.3.4:7777"

	newNotifier := func(sender *captureSender) *nodeevents.MailNotifier {
		return nodeevents.NewMailNotifier(zaptest.NewLogger(t), sender)
	}

	t.Run("sends one email per batch", func(t *testing.T) {
		sender := &captureSender{}

		require.NoError(t, newNotifier(sender).Notify(ctx, "saltlake", []nodeevents.NodeEvent{
			{Email: "sno@storj.test", NodeID: node1, Event: nodeevents.Offline, LastIPPort: &ipPort},
			{Email: "sno@storj.test", NodeID: node2, Event: nodeevents.Offline},
			// a repeated node ID must be listed once.
			{Email: "sno@storj.test", NodeID: node1, Event: nodeevents.Offline, LastIPPort: &ipPort},
		}))

		messages := sender.messages()
		require.Len(t, messages, 1)

		email, ok := messages[0].(*nodeevents.NodeEventEmail)
		require.True(t, ok)
		require.Equal(t, nodeevents.Offline, email.Event)
		require.Equal(t, "saltlake", email.Satellite)
		require.Equal(t, []nodeevents.NodeSummary{
			{ID: node1.String(), Address: ipPort},
			{ID: node2.String()},
		}, email.Nodes)
	})

	t.Run("caps the listed nodes", func(t *testing.T) {
		sender := &captureSender{}
		events := make([]nodeevents.NodeEvent, 150)
		for i := range events {
			events[i] = nodeevents.NodeEvent{Email: "sno@storj.test", NodeID: testrand.NodeID(), Event: nodeevents.Offline}
		}
		require.NoError(t, newNotifier(sender).Notify(ctx, "saltlake", events))

		messages := sender.messages()
		require.Len(t, messages, 1)
		email := messages[0].(*nodeevents.NodeEventEmail)
		require.Len(t, email.Nodes, 100)
		require.Equal(t, 50, email.More)
	})

	t.Run("empty batch does nothing", func(t *testing.T) {
		sender := &captureSender{}
		require.NoError(t, newNotifier(sender).Notify(ctx, "saltlake", nil))
		require.Empty(t, sender.messages())
	})

	t.Run("invalid event type sends nothing", func(t *testing.T) {
		sender := &captureSender{}
		require.Error(t, newNotifier(sender).Notify(ctx, "saltlake", []nodeevents.NodeEvent{
			{Email: "sno@storj.test", NodeID: node1, Event: nodeevents.Type(99)},
		}))
		require.Empty(t, sender.messages())
	})

	t.Run("send failure is returned to the chore", func(t *testing.T) {
		sender := &captureSender{err: errMailFailure}
		err := newNotifier(sender).Notify(ctx, "saltlake", []nodeevents.NodeEvent{
			{Email: "sno@storj.test", NodeID: node1, Event: nodeevents.Offline},
		})
		require.ErrorIs(t, err, errMailFailure)
	})
}

func TestNewNotifier(t *testing.T) {
	log := zaptest.NewLogger(t)
	sender := &captureSender{}

	for _, tt := range []struct {
		configured string
		expected   nodeevents.Notifier
	}{
		{"customer.io", &nodeevents.CustomerioNotifier{}},
		{"mail", &nodeevents.MailNotifier{}},
		{"", &nodeevents.MockNotifier{}},
		{"hubspot", &nodeevents.MockNotifier{}},
	} {
		notifier := nodeevents.NewNotifier(log, nodeevents.Config{Notifier: tt.configured}, sender)
		require.IsType(t, tt.expected, notifier, "notifier %q", tt.configured)
	}

	t.Run("warns when emails are enabled but the notifier is unrecognized", func(t *testing.T) {
		observed, logs := observer.New(zapcore.WarnLevel)
		cfg := nodeevents.Config{Notifier: "customerio", SendNodeEmails: true}
		nodeevents.NewNotifier(zap.New(observed), cfg, sender)
		require.Equal(t, 1, logs.Len(), "a misconfigured notifier must not fail silently")

		// no warning when the chore is not sending anything anyway.
		observed, logs = observer.New(zapcore.WarnLevel)
		cfg.SendNodeEmails = false
		nodeevents.NewNotifier(zap.New(observed), cfg, sender)
		require.Zero(t, logs.Len())
	})
}

func TestMailNotifierRendersAllTemplates(t *testing.T) {
	ctx := testcontext.New(t)

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	templatePath := filepath.Join(filepath.Dir(thisFile), "..", "..", "web", "satellite", "static", "emails")

	sender := &capturePostSender{}
	branding := mailservice.WhiteLabelConfig{
		BrandName:    "Storj",
		DocsURL:      "https://storj.dev/docs",
		SupportURL:   "https://supportdcs.storj.io",
		CompanyName:  "Storj Labs",
		PrimaryColor: "#0052FF",
	}
	mail, err := mailservice.New(zaptest.NewLogger(t), sender, templatePath, mailservice.TenantConfig{}, branding, nil)
	require.NoError(t, err)
	defer ctx.Check(mail.Close)

	notifier := nodeevents.NewMailNotifier(zaptest.NewLogger(t), mail)

	node1, node2 := testrand.NodeID(), testrand.NodeID()
	ipPort := "1.2.3.4:7777"

	templates := make(map[string]struct{})

	for _, eventType := range allEventTypes {
		name, err := eventType.Name()
		require.NoError(t, err)

		t.Run(name, func(t *testing.T) {
			template := (&nodeevents.NodeEventEmail{Event: eventType}).Template()
			_, duplicate := templates[template]
			require.False(t, duplicate, "template %q is already used by another event type", template)
			templates[template] = struct{}{}

			require.NoError(t, notifier.Notify(ctx, "saltlake", []nodeevents.NodeEvent{
				{Email: "sno@storj.test", NodeID: node1, Event: eventType, LastIPPort: &ipPort},
				{Email: "sno@storj.test", NodeID: node2, Event: eventType},
			}))

			msg := sender.last()
			require.NotNil(t, msg)
			require.Equal(t, []post.Address{{Address: "sno@storj.test"}}, msg.To)
			require.Contains(t, msg.Subject, "Storj - Storage node")
			require.Contains(t, msg.PlainText, node1.String())
			require.Len(t, msg.Parts, 1)

			// assert without require.Contains so a failure does not dump the whole email.
			has := func(want string) bool { return strings.Contains(msg.Parts[0].Content, want) }

			require.True(t, has(node1.String()) && has(node2.String()), "node IDs missing")
			require.True(t, has(ipPort), "node address missing")
			require.True(t, has("saltlake"), "satellite name missing")
			require.True(t, has(branding.DocsURL+`"`) || has(branding.SupportURL+`"`), "no call to action")
			require.False(t, has(node2.String()+" &mdash;"), "node without an address got a dangling separator")
			require.False(t, has("{{"), "unrendered template action")
		})
	}
}
