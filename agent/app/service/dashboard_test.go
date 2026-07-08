package service

import (
	"errors"
	"testing"

	"github.com/1Panel-dev/1Panel/agent/app/dto"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
)

func TestApplyCPUInfoHandlesEmptyInfo(t *testing.T) {
	var base dto.DashboardBase

	applyCPUInfo(&base, nil)

	if base.CPUModelName != "" {
		t.Fatalf("expected empty CPU model for empty info, got %q", base.CPUModelName)
	}
	if base.CPUMhz != 0 {
		t.Fatalf("expected zero CPU MHz for empty info, got %v", base.CPUMhz)
	}
}

func TestApplyCPUInfoUsesFirstCPUInfo(t *testing.T) {
	var base dto.DashboardBase

	applyCPUInfo(&base, []cpu.InfoStat{{ModelName: "Example CPU", Mhz: 2400}})

	if base.CPUModelName != "Example CPU" {
		t.Fatalf("expected CPU model to be copied, got %q", base.CPUModelName)
	}
	if base.CPUMhz != 2400 {
		t.Fatalf("expected CPU MHz to be copied, got %v", base.CPUMhz)
	}
}

func TestApplyLoadInfoHandlesEmptyInfo(t *testing.T) {
	current := dashboardLoadInfoTarget{cpuTotal: 4}

	applyLoadInfo(&current, nil)

	if current.load1 != 0 || current.loadUsagePercent != 0 {
		t.Fatalf("expected empty load info to keep zero values, got %#v", current)
	}
}

func TestApplyLoadInfoCalculatesUsageWhenCPUTotalExists(t *testing.T) {
	current := dashboardLoadInfoTarget{cpuTotal: 4}

	applyLoadInfo(&current, &load.AvgStat{Load1: 3, Load5: 2, Load15: 1})

	if current.load1 != 3 || current.load5 != 2 || current.load15 != 1 {
		t.Fatalf("expected load averages to be copied, got %#v", current)
	}
	if current.loadUsagePercent != 50 {
		t.Fatalf("expected load usage 50, got %v", current.loadUsagePercent)
	}
}

func TestBuildDiskInfoFromPartitionsKeepsUsableWindowsVolumes(t *testing.T) {
	partitions := []disk.PartitionStat{
		{Device: "C:", Mountpoint: "C:\\", Fstype: "NTFS"},
		{Device: "D:", Mountpoint: "D:\\", Fstype: "NTFS"},
		{Device: "cdrom", Mountpoint: "E:\\", Fstype: "CDFS"},
	}

	items := buildDiskInfoFromPartitions(partitions, func(path string) (*disk.UsageStat, error) {
		if path == "D:\\" {
			return nil, errors.New("drive not ready")
		}
		return &disk.UsageStat{
			Path:              path,
			Total:             100,
			Free:              40,
			Used:              60,
			UsedPercent:       60,
			InodesTotal:       10,
			InodesUsed:        3,
			InodesFree:        7,
			InodesUsedPercent: 30,
		}, nil
	})

	if len(items) != 2 {
		t.Fatalf("expected 2 usable disk items, got %d: %#v", len(items), items)
	}
	if items[0].Path != "C:\\" || items[0].Total != 100 || items[0].UsedPercent != 60 {
		t.Fatalf("unexpected first disk item: %#v", items[0])
	}
	if items[1].Path != "D:\\" || items[1].Total != 0 {
		t.Fatalf("expected unavailable disk to keep base metadata only, got %#v", items[1])
	}
}
