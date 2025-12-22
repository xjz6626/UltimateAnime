package pikpak

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// 复刻 Python 版 pikpakapi/utils.py 的 Salt
var PythonSalts = []string{
	"Gez0T9ijiI9WCeTsKSg3SMlx",
	"zQdbalsolyb1R/",
	"ftOjr52zt51JD68C3s",
	"yeOBMH0JkbQdEFNNwQ0RI9T3wU/v",
	"BRJrQZiTQ65WtMvwO",
	"je8fqxKPdQVJiy1DM6Bc9Nb1",
	"niV",
	"9hFCW2R1",
	"sHKHpe2i96",
	"p7c5E6AcXQ/IJUuAEC9W6",
	"",
	"aRv9hjc9P+Pbn+u3krN6",
	"BzStcgE8qVdqjEH16l4",
	"SqgeZvL5j9zoHP95xWHt",
	"zVof5yaJkPe3VFpadPof",
}

const (
	AndroidClientID      = "YNxT9w7GMdWvEOKa"
	AndroidClientSecret  = "dbw2OtmVEeuUvIptb1Coyg"
	AndroidClientVersion = "1.47.1" // 对齐 Python
	AndroidPackageName   = "com.pikcloud.pikpak"
	AndroidSdkVersion    = "2.0.4.204000" // 对齐 Python
)

func generateDeviceSign(deviceID, packageName string) string {
	signatureBase := fmt.Sprintf("%s%s%s%s", deviceID, packageName, "1", "appkey")
	sha1Hash := sha1.New()
	sha1Hash.Write([]byte(signatureBase))
	sha1Result := sha1Hash.Sum(nil)
	sha1String := hex.EncodeToString(sha1Result)
	md5Hash := md5.New()
	md5Hash.Write([]byte(sha1String))
	md5Result := md5Hash.Sum(nil)
	md5String := hex.EncodeToString(md5Result)
	return fmt.Sprintf("div101.%s%s", deviceID, md5String)
}

func BuildCustomUserAgent(deviceID, clientID, appName, sdkVersion, clientVersion, packageName, userID string) string {
	deviceSign := generateDeviceSign(deviceID, packageName)
	var sb strings.Builder
	// 严格复刻 Python user-agent 格式
	sb.WriteString(fmt.Sprintf("ANDROID-%s/%s ", appName, clientVersion))
	sb.WriteString("protocolVersion/200 ")
	sb.WriteString("accesstype/ ")
	sb.WriteString(fmt.Sprintf("clientid/%s ", clientID))
	sb.WriteString(fmt.Sprintf("clientversion/%s ", clientVersion))
	sb.WriteString("action_type/ ")
	sb.WriteString("networktype/WIFI ")
	sb.WriteString("sessionid/ ")
	sb.WriteString(fmt.Sprintf("deviceid/%s ", deviceID))
	sb.WriteString("providername/NONE ")
	sb.WriteString(fmt.Sprintf("devicesign/%s ", deviceSign))
	sb.WriteString("refresh_token/ ")
	sb.WriteString(fmt.Sprintf("sdkversion/%s ", sdkVersion))
	sb.WriteString(fmt.Sprintf("datetime/%d ", time.Now().UnixMilli()))
	sb.WriteString(fmt.Sprintf("usrno/%s ", userID))
	sb.WriteString(fmt.Sprintf("appname/%s ", appName)) // Python 这里是 appname/com.pikcloud.pikpak
	sb.WriteString("session_origin/ ")
	sb.WriteString("grant_type/ ")
	sb.WriteString("appid/ ")
	sb.WriteString("clientip/ ")
	sb.WriteString("devicename/Xiaomi_M2004j7ac ")
	sb.WriteString("osversion/13 ")
	sb.WriteString("platformversion/10 ")
	sb.WriteString("accessmode/ ")
	sb.WriteString("devicemodel/M2004J7AC")
	return sb.String()
}

// GetCaptchaSign 获取验证码签名 (Python 算法)
func GetCaptchaSign(clientID, clientVersion, packageName, deviceID string) (timestamp, sign string) {
	timestamp = fmt.Sprint(time.Now().UnixMilli())
	str := fmt.Sprint(clientID, clientVersion, packageName, deviceID, timestamp)
	for _, salt := range PythonSalts {
		str = md5Str(str + salt)
	}
	sign = "1." + str
	return
}

func md5Str(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
