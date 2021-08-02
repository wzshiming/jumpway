package i18n

import (
	locale "github.com/Xuanwo/go-locale"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	_ "golang.org/x/text/message/catalog"
)

//go:generate gotext -srclang=en update -out=catalog.go -lang=en,zh

var (
	fmt *message.Printer
)

func init() {
	fmt = message.NewPrinter(GetLocale())
}

func GetLocale() language.Tag {
	tag, err := locale.Detect()
	if err != nil {
		return language.English
	}
	return tag
}

func Daemon() string {
	return fmt.Sprintf("Daemon")
}

func ManualProxy() string {
	return fmt.Sprintf("Manual Proxy")
}

func SystemProxy() string {
	return fmt.Sprintf("System Proxy")
}

func ProxyMode() string {
	return fmt.Sprintf("Proxy Mode")
}

func ExportCommand() string {
	return fmt.Sprintf("Export Command")
}

func Config() string {
	return fmt.Sprintf("Config")
}

func EditConfig() string {
	return fmt.Sprintf("Edit Config")
}

func ReloadConfig() string {
	return fmt.Sprintf("Reload Config")
}

func ViewEditConfig() string {
	return fmt.Sprintf("View Edit Config")
}

func Log() string {
	return fmt.Sprintf("Log")
}

func About() string {
	return fmt.Sprintf("About")
}

func Quit() string {
	return fmt.Sprintf("Quit")
}

func RedirectLog() string {
	return fmt.Sprintf("Redirect Log")
}

func InitConfig() string {
	return fmt.Sprintf("Init Config")
}

func WriteClipboard() string {
	return fmt.Sprintf("Write Clipboard")
}

func RunProxy() string {
	return fmt.Sprintf("Run Proxy")
}

func Args() string {
	return fmt.Sprintf("Args")
}

func OpenFile() string {
	return fmt.Sprintf("Open File")
}

func Connect() string {
	return fmt.Sprintf("Connect")
}

func UseProxy() string {
	return fmt.Sprintf("Use Proxy")
}

func Listen(address string) string {
	return fmt.Sprintf("Listen %s", address)
}

func Alert(message string) string {
	return fmt.Sprintf("Alert %s", message)
}

func Status(mode string, address string) string {
	return fmt.Sprintf("JumpWay %s On %s", mode, address)
}
