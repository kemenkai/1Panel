package router

func commonGroups() []CommonRouter {
	return []CommonRouter{
		&BaseRouter{},
		&BackupRouter{},
		&LogRouter{},
		&SettingRouter{},
		&SimpleNodeRouter{},
		&CommandRouter{},
		&GroupRouter{},
		&ScriptRouter{},
	}
}
