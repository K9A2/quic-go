package quic

import (
  "encoding/json"
  "fmt"
  "io/ioutil"
  "os"
)

const configFileName = "managed-streams.json"

// var youtubeList = []string{
// 	"index.html",
// 	"desktop_polymer_v2-vflfD_pIA.html",
// 	"www-onepick-2x-webp-vflsYL2Tr.css",
// 	"YTSans300500700.css",
// 	"RobotoMono400700.css",
// 	"www-main-desktop-watch-page-skeleton-2x-webp-vflQ9GNSj.css",
// 	"www-main-desktop-home-page-skeleton-2x-webp-vfl4iT_wE.css",
// 	"Roboto400300300italic400italic500500italic700700italic.css",
// 	"www-player-2x-webp.css",
// 	"www-prepopulator.js",
// 	"custom-elements-es5-adapter.js",
// 	"scheduler.js",
// 	"www-tampering.js",
// 	"network.js",
// 	"spf.js",
// 	"web-animations-next-lite.min.js",
// 	"webcomponents-sd.js",
// 	"base.js",
// 	"desktop_polymer_v2.js",
// 	"KFOmCnqEu92Fr1Mu4mxK.woff2",
// 	"KFOlCnqEu92Fr1MmEU9fBBc4.woff2",
// }

// SequenceFile 是 JSON 格式的传输顺序文件
type SequenceFile struct {
  ManagedStreams []string `json:"managedStreams"`
}

// LoadConfig 读取文件传输顺序，成功读取时会返回解析后的文件顺序数据
func LoadConfig() *SequenceFile {
  jsonFile, err := os.Open(configFileName)
  if err != nil {
    fmt.Printf("error in loading file sequence, err = <%s>\n", err.Error())
    return nil
  }
  defer jsonFile.Close()

  byteValue, _ := ioutil.ReadAll(jsonFile)
  var sequence SequenceFile
  json.Unmarshal(byteValue, &sequence)
  return &sequence
}
