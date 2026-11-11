package mod

import "testing"

// B17/Windows: steamcmd 下载参数须单独 argv(防 shell 注入、含空格路径断裂),
// 且 +workshop_download_item 的 appid/modid 须分离(非捆绑成一个 argv)。

func TestSteamCmdDownloadArgsSeparatesTokens(t *testing.T) {
	args := steamCmdDownloadArgs(`C:\steam cmd`, "99999")

	// 含空格路径须作为单一 argv(不被分割)
	foundPath := false
	for _, a := range args {
		if a == `C:\steam cmd` {
			foundPath = true
		}
	}
	if !foundPath {
		t.Fatalf("含空格路径须作为单一 argv,实际 %v", args)
	}

	// mod_id 须作为单一 argv
	foundId := false
	for _, a := range args {
		if a == "99999" {
			foundId = true
		}
	}
	if !foundId {
		t.Fatalf("mod_id 须作为单一 argv,实际 %v", args)
	}

	// +workshop_download_item 后 appid 与 modid 须分离(禁止捆绑成 "…322330 99999")
	for i, a := range args {
		if a == "+workshop_download_item" {
			if i+2 >= len(args) {
				t.Fatalf("+workshop_download_item 后缺 appid/modid argv,实际 %v", args)
			}
			if args[i+1] != "322330" {
				t.Fatalf("+workshop_download_item 后 argv 应为 appid 322330,实际 %q", args[i+1])
			}
			if args[i+2] != "99999" {
				t.Fatalf("+workshop_download_item 后 argv 应为 modid 99999,实际 %q", args[i+2])
			}
			return
		}
	}
	t.Fatalf("args 缺 +workshop_download_item,实际 %v", args)
}
