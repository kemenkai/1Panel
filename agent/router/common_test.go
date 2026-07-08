package router

import "testing"

func TestCommonGroupsIncludesWindowsServiceRouter(t *testing.T) {
	routers := commonGroups()
	for _, item := range routers {
		if _, ok := item.(*WindowsServiceRouter); ok {
			return
		}
	}
	t.Fatal("WindowsServiceRouter is not registered in commonGroups")
}
