// Island Adventures (workshop-1467214795) 世界预设
// 模板数据来源：鸽子鸽 lv.137-3 逆向还原（Shipwrecked 完整 168 项生成配置）
package levelConfigUtils

import (
	_ "embed"
	"dst-admin-go/vo/level"
	"fmt"
)

//go:embed templates/shipwrecked_leveldataoverride.lua
var shipwreckedLevelData string

//go:embed templates/volcano_leveldataoverride.lua
var volcanoLevelData string

const IAModId = "workshop-1467214795"

// iaModoverrides 启用 IA 本体并提供互通开关（鸽子鸽 no_joining_islands）
func iaModoverrides(noJoiningIslands bool) string {
	join := "true"
	if !noJoiningIslands {
		join = "false"
	}
	return `return {
  ["` + IAModId + `"] = {
    enabled = true,
    configuration_options = {
      no_joining_islands = ` + join + `,
    },
  },
}
`
}

// GenerateIAPreset 生成 IA 两世界定义。
// overlay: 追加到现有存档，两世界均不抢主分片；
// standalone: 独立 IA 存档，Shipwrecked 为主分片。
func GenerateIAPreset(mode string) []level.World {
	var swMaster bool
	switch mode {
	case "overlay":
		swMaster = false
	case "standalone":
		swMaster = true
	default:
		return nil
	}
	sw := level.World{
		LevelName:         "Shipwrecked",
		IsMaster:          swMaster,
		Uuid:              fmt.Sprintf("ia-shipwrecked-%s", mode),
		Leveldataoverride: shipwreckedLevelData,
		Modoverrides:      iaModoverrides(true),
		ServerIni: &level.ServerIni{
			Name: "Shipwrecked",
		},
	}
	vol := level.World{
		LevelName:         "Volcano",
		IsMaster:          false,
		Uuid:              fmt.Sprintf("ia-volcano-%s", mode),
		Leveldataoverride: volcanoLevelData,
		Modoverrides:      iaModoverrides(true),
		ServerIni: &level.ServerIni{
			Name: "Volcano",
		},
	}
	return []level.World{sw, vol}
}
