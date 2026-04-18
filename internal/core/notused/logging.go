package notused

import (
	"ceph-go-test/internal/utils"
	"go.uber.org/zap"
)

var slog *zap.SugaredLogger

func init() {
	//slog = utils.GetLogger().With(zap.String("package", "notused")).Sugar()
	slog = utils.GetLogger()
}
