package rbd

import (
	"ceph-core-api/internal/utils"

	"go.uber.org/zap"
)

var logger *zap.SugaredLogger

func init() {
	logger = utils.GetLogger()
	//logger = utils.GetLogger().With(zap.String("package", "rbd"))
}
