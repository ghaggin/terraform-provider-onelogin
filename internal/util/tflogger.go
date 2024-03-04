package util

import (
	"context"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// TFLogger is a wrapper around the terraform logger
// to implement the onelogin.Logger interface
type TFLogger struct {
}

func (*TFLogger) Trace(ctx context.Context, msg string, fields ...map[string]interface{}) {
	tflog.Trace(ctx, msg, fields...)

}
func (*TFLogger) Debug(ctx context.Context, msg string, fields ...map[string]interface{}) {
	tflog.Debug(ctx, msg, fields...)
}

func (*TFLogger) Info(ctx context.Context, msg string, fields ...map[string]interface{}) {
	tflog.Info(ctx, msg, fields...)
}

func (*TFLogger) Warn(ctx context.Context, msg string, fields ...map[string]interface{}) {
	tflog.Warn(ctx, msg, fields...)
}

func (*TFLogger) Error(ctx context.Context, msg string, fields ...map[string]interface{}) {
	tflog.Error(ctx, msg, fields...)
}
