package extensions

import (
	"fmt"
	"testing"
)

func TestReadSyncKeepsSupportedVersionsAndUnicode(t *testing.T) {
	for _, version := range []int{1, 2} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			data := []byte(fmt.Sprintf(`{"version":%d,"extensions":[{"id":"dev.workspace","config":{"label":"项目 � 工具"}}]}`, version))
			doc, err := ReadSync(data)
			if err != nil || len(doc.Extensions) != 1 || doc.Extensions[0].Config["label"] != "项目 � 工具" {
				t.Fatalf("valid UTF-8/version changed: %+v err=%v", doc, err)
			}
		})
	}
}
