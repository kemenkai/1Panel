package buildinfo

// Version 为编译期通过 ldflags 注入的版本号。
// 注入方式: -X github.com/1Panel-dev/1Panel/core/buildinfo.Version=<版本>
// 未注入时为空字符串，启动自愈逻辑会跳过（保持对官方旧构建的兼容）。
var Version string
