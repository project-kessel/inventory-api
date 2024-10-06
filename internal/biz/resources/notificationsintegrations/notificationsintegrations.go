package notificationsintegrations

import (
	"github.com/project-kessel/inventory-api/internal/biz"
)

const (
	ResourceType = "notifications-integration"
)

type NotificationsIntegrationUsecase = biz.DefaultUsecase[NotificationsIntegration, string]

var New = biz.New[NotificationsIntegration, string]
