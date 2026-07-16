package hook

import (
	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/app/repo"
	"github.com/1Panel-dev/1Panel/core/buildinfo"
	"github.com/1Panel-dev/1Panel/core/global"
)

// SyncCompiledVersion 以「正在运行的二进制版本」为准，将 SystemVersion 写回数据库。
//
// 背景: 侧边栏版本号读取 core.db 的 settings 表 SystemVersion 字段（Init 中读入
// global.CONF.Base.Version）。手动替换二进制不会更新该值，导致升级后版本号停留在旧版。
//
// 语义: 官方在线升级路径先写 DB 再换同版本二进制，故 compiled == db 时本函数 no-op，
// 不会干扰官方升级流程；仅在「二进制版本与 DB 记录不一致」时自愈。
func SyncCompiledVersion() {
	compiled := buildinfo.Version
	if len(compiled) == 0 {
		// 未注入版本的旧构建不做任何动作。
		return
	}

	settingRepo := repo.NewISettingRepo()
	current, err := settingRepo.GetValueByKey("SystemVersion")
	if err != nil {
		global.LOG.Errorf("self-heal SystemVersion: read current version failed, err: %v", err)
		return
	}
	if current == compiled {
		return
	}

	// 双写 core.db 与 agent.db，镜像 core/app/service/upgrade.go 的升级写法，保证即时生效。
	if err := settingRepo.Update("SystemVersion", compiled); err != nil {
		global.LOG.Errorf("self-heal SystemVersion: update core.db failed, err: %v", err)
		return
	}
	if global.AgentDB != nil {
		if err := global.AgentDB.Model(&model.Setting{}).Where("key = ?", "SystemVersion").
			Updates(map[string]interface{}{"value": compiled}).Error; err != nil {
			global.LOG.Errorf("self-heal SystemVersion: update agent.db failed, err: %v", err)
		}
	}
	global.CONF.Base.Version = compiled
	global.LOG.Infof("self-heal SystemVersion: %s -> %s", current, compiled)
}
