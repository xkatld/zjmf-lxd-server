package services

import (
	"context"
	"os/exec"

	"lxdapi/pkg/logger"

	"go.uber.org/zap"
)

type IptablesPersistentManager struct {
	enabled bool
}

var PersistentManager *IptablesPersistentManager

func InitIptablesPersistent() error {
	PersistentManager = &IptablesPersistentManager{
		enabled: true,
	}
	return nil
}

func (m *IptablesPersistentManager) SaveRules() {
	if !m.enabled {
		return
	}

	ctx := context.Background()
	lc := &logger.Context{
		Action: "save_iptables_rules",
	}
	ctx = logger.NewContext(ctx, lc)

	if err := m.saveIPv4Rules(); err != nil {
		logger.Global.Warn(ctx, "保存IPv4规则失败", zap.Error(err))
	}

	if err := m.saveIPv6Rules(); err != nil {
		logger.Global.Warn(ctx, "保存IPv6规则失败", zap.Error(err))
	}
}

func (m *IptablesPersistentManager) saveIPv4Rules() error {
	cmd := exec.Command("sh", "-c", "iptables-save > /etc/iptables/rules.v4")
	return cmd.Run()
}

func (m *IptablesPersistentManager) saveIPv6Rules() error {
	cmd := exec.Command("sh", "-c", "ip6tables-save > /etc/iptables/rules.v6")
	return cmd.Run()
}

func (m *IptablesPersistentManager) IsEnabled() bool {
	return m != nil && m.enabled
}
