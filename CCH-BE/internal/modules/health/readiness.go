package health

import "context"

type ReadinessChecker interface {
	PingContext(context.Context) error
}
