package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	UserList []User `mapstructure:"User-list"  yaml:"User-list"`

	Timer struct {
		Start bool
		Spec  string
	}

	Dingtalk struct {
		AccessToken string `mapstructure:"accessToken"  yaml:"accessToken"`
		SecretKey   string `mapstructure:"secretKey"  yaml:"secretKey"`
	}

	//Retry struct {
	//	Attempts  int `attempts:"accessToken"  yaml:"attempts"`
	//	Sleeptime int `attempts:"sleeptime"  yaml:"sleeptime"`
	//}
}

type User struct {
	NickName string
	SignType string
	OpenID   string
	UnionID  string
	Province string
	Country  string
	City     string
	//执行结果
	UserName      string
	ErrorMsg      string
	IsSignSuccess bool
}

type Result struct {
	Users []User
	Date  time.Time
}

func main() {

	var cstZone = time.FixedZone("CST", 8*3600)
	time.Local = cstZone
	fmt.Println("现在系统时间:", time.Now())

	//读取配置文件
	viper.SetConfigName("config") // name of config file (without extension)
	viper.SetConfigType("yaml")   // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(".")      // optionally look for config in the working directory
	err := viper.ReadInConfig()   // Find and read the config file
	if err != nil {               // Handle errors reading the config file
		fmt.Println("config.yaml 配置文件不存在!")
		return
	}
	var config Config
	viper.Unmarshal(&config)

	//设置定时器
	task := NewTimerTask()
	_, err = task.AddTaskByFunc("signTask", config.Timer.Spec, func() {
		TimedTask(config)
	})

	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()

}

/*
	1.登录
	2.获取用户信息
	3.获取TrainID
	4.获取签到信息
	5.提交签到信息
	6.签到
*/

func TimedTask(config Config) {

	signType := 0
	var results Result

	//判断上下午
	if time.Now().Hour() < 12 {
		signType = SIGNIN
	} else {
		signType = SIGNOUT
	}
	results.Date = time.Now()

	fmt.Printf("########## 触发时间 %s ##########\n", results.Date.Format("2006-01-02 15:04:05"))

	for _, user := range config.UserList {
		fmt.Printf("执行 %s 签到\n", user.NickName)
		service := XybService{}
		err := service.Login(user.SignType, user.OpenID, user.UnionID)
		if err != nil {
			user.UserName = user.NickName
			user.ErrorMsg = "登录失败"
			results.Users = append(results.Users, user)
			fmt.Printf("%s err:%s\n", user.ErrorMsg, err.Error())
			continue
		}

		name, err := service.GetNickName()
		if err != nil {
			user.UserName = user.NickName
			user.ErrorMsg = "获取个人信息失败"
			results.Users = append(results.Users, user)
			fmt.Printf("%s err:%s\n", user.ErrorMsg, err.Error())
			continue
		}

		user.UserName = name

		id, err := service.GetTrainID()
		if err != nil {
			user.ErrorMsg = "获取TrainID失败"
			results.Users = append(results.Users, user)
			fmt.Printf("%s err:%s\n", user.ErrorMsg, err.Error())
			continue
		}

		message, err := service.GetSignMessage(id)
		if err != nil {
			user.ErrorMsg = "获取签到信息失败"
			results.Users = append(results.Users, user)
			fmt.Printf("%s err:%s\n", user.ErrorMsg, err.Error())
			continue
		}

		//检查上午是否未签
		if signType == SIGNIN {
			if message.Data.ClockInfo.Status != 2 {
				user.ErrorMsg = "签到状态异常或用户已签到"
				results.Users = append(results.Users, user)
				continue
			}
		} else {
			if message.Data.ClockInfo.Status != 1 {
				user.ErrorMsg = "签到状态异常或用户已签到"
				results.Users = append(results.Users, user)
				continue
			}
		}

		code, err := service.GetCityCode(user.City)
		if err != nil {
			user.ErrorMsg = "获取城市代码失败"
			results.Users = append(results.Users, user)
			fmt.Printf("%s err:%s\n", user.ErrorMsg, err.Error())
			continue
		}
		//执行签到
		err = service.Sign(SignData{
			NickName: user.UserName,
			Location: struct {
				AdCode    string
				Longitude string
				Latitude  string
				Province  string
				Country   string
				City      string
				Address   string
			}{
				Country:   user.Country,
				Province:  user.Province,
				City:      user.City,
				Address:   message.Data.PostInfo.Address,
				Longitude: fmt.Sprintf("%f", message.Data.PostInfo.Lng),
				Latitude:  fmt.Sprintf("%f", message.Data.PostInfo.Lat),
				AdCode:    code,
			},
		}, signType)
		if err != nil {
			user.ErrorMsg = "执行签到失败"
			results.Users = append(results.Users, user)
			fmt.Printf("%s err:%s\n", user.ErrorMsg, err.Error())
			continue
		}
		user.IsSignSuccess = true
		results.Users = append(results.Users, user)
		time.Sleep(time.Second * 3)
	}
	DingtalkRobot(config.Dingtalk.AccessToken, config.Dingtalk.SecretKey, results)
}

func DingtalkRobot(accessToken, secret string, resuilt Result) {

	body := fmt.Sprintf("📢签到日志📢 \n###### 触发时间:%s\n", resuilt.Date.Format("2006-01-02 15:04:05"))

	for _, user := range resuilt.Users {
		var isSuccess string
		if user.IsSignSuccess {
			isSuccess = "🎉 签到成功"
		} else {
			isSuccess = "❌"
		}
		body = body + fmt.Sprintf(" ##### %s %s %s\n", user.UserName, isSuccess, user.ErrorMsg)
	}

	bodyMap := make(map[string]interface{})
	bodyMap["msgtype"] = "markdown"
	makedownMap := make(map[string]string)
	makedownMap["title"] = "签到日志"
	makedownMap["text"] = body
	bodyMap["markdown"] = makedownMap

	jsonBody, _ := json.Marshal(bodyMap)

	url := Signature(accessToken, secret)
	payload := strings.NewReader(string(jsonBody))

	client := &http.Client{}
	req, _ := http.NewRequest("POST", url, payload)
	req.Header.Add("Content-Type", "application/json")
	res, _ := client.Do(req)
	repbody, _ := ioutil.ReadAll(res.Body)

	fmt.Println(string(repbody))

	defer res.Body.Close()
}

func Signature(accessToken, secret string) string {

	webhookurl := "https://oapi.dingtalk.com/robot/send?access_token=" + accessToken
	// 获取当前秒级时间戳
	timestamp := time.Now()
	milliTimestamp := timestamp.UnixNano() / 1e6
	stringToSign := fmt.Sprintf("%s\n%s", strconv.Itoa(int(milliTimestamp)), secret)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stringToSign))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	hookurl := fmt.Sprintf("%s&timestamp=%s&sign=%s", webhookurl, strconv.Itoa(int(milliTimestamp)), sign)
	return hookurl
}
