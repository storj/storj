// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeevents

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/private/post"
	"storj.io/storj/satellite/mailservice"
)

// MailSender sends rendered emails. *mailservice.Service satisfies this interface.
type MailSender interface {
	SendRendered(ctx context.Context, to []post.Address, msg mailservice.Message) error
}

// MailNotifier notifies node operators about node events with emails rendered
// and sent by the satellite's own mail service.
type MailNotifier struct {
	log  *zap.Logger
	mail MailSender
}

// NewMailNotifier is a constructor for MailNotifier.
func NewMailNotifier(log *zap.Logger, mail MailSender) *MailNotifier {
	return &MailNotifier{
		log:  log,
		mail: mail,
	}
}

// Notify emails a node operator about a batch of events which occurred on their nodes.
func (m *MailNotifier) Notify(ctx context.Context, satellite string, events []NodeEvent) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(events) == 0 {
		return nil
	}

	email := events[0].Email
	eventType := events[0].Event
	eventName, err := eventType.Name()
	if err != nil {
		return err
	}
	if _, ok := nodeEventEmails[eventType]; !ok {
		return Error.New("no email template for event %q", eventName)
	}

	nodes := make([]NodeSummary, 0, len(events))
	seen := make(map[storj.NodeID]struct{})
	for _, e := range events {
		if _, ok := seen[e.NodeID]; ok {
			continue
		}
		seen[e.NodeID] = struct{}{}

		node := NodeSummary{ID: e.NodeID.String()}
		if e.LastIPPort != nil {
			node.Address = *e.LastIPPort
		}
		nodes = append(nodes, node)
	}

	total := len(nodes)
	if total > maxNodesPerEmail {
		nodes = nodes[:maxNodesPerEmail]
	}

	// send synchronously: the chore needs the error to decide whether the
	// batch is marked as sent or retried later.
	err = m.mail.SendRendered(ctx, []post.Address{{Address: email}}, &NodeEventEmail{
		Event:     eventType,
		Satellite: satellite,
		Nodes:     nodes,
		More:      total - len(nodes),
	})
	if err != nil {
		return Error.Wrap(err)
	}

	m.log.Info("node event email sent",
		zap.String("email", email),
		zap.String("event", eventName),
		zap.Int("nodes", total))

	return nil
}
