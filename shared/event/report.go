// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package event

import (
	"context"
	"fmt"
	"time"

	"storj.io/eventkit"
)

var evs = eventkit.Package()

// Report sends all annotations from the context as an eventkit event.
func Report(ctx context.Context) {
	val := ctx.Value(annotationKey)
	if val == nil {
		return
	}
	annotations, ok := val.(*Annotations)
	if !ok {
		return
	}

	var tags []eventkit.Tag
	name := "main"
	annotations.ForEach(func(key string, value interface{}) {
		if key == nameKey {
			name = fmt.Sprintf("%v", value)
			return
		}
		switch v := value.(type) {
		case string:
			tags = append(tags, eventkit.String(key, v))
		case []byte:
			tags = append(tags, eventkit.Bytes(key, v))
		case time.Time:
			tags = append(tags, eventkit.Timestamp(key, v))
		case time.Duration:
			tags = append(tags, eventkit.Duration(key, v))
		case float64:
			tags = append(tags, eventkit.Float64(key, v))
		case int:
			tags = append(tags, eventkit.Int64(key, int64(v)))
		case int64:
			tags = append(tags, eventkit.Int64(key, v))
		case bool:
			tags = append(tags, eventkit.Bool(key, v))
		}
	})
	evs.Event(name, tags...)
}
