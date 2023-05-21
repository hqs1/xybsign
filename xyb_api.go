package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-resty/resty/v2"
	"time"
)

/*
	1.登录
	2.获取用户信息
	3.获取TrainID
	4.获取签到信息
	5.提交签到信息
	6.签到
*/

const (
	LOGIN_API       = "https://xcx.xybsyw.com/login/login.action"
	WXLOGIN_API     = "https://xcx.xybsyw.com/login/login!wx.action"
	CITYCODE_API    = "https://xcx.xybsyw.com/common/loadLocation!getCityId.action"
	UserMESSAGE_API = "https://xcx.xybsyw.com/account/LoadAccountInfo.action"
	SIGNMESSAGE_API = "https://xcx.xybsyw.com/student/clock/GetPlan!detail.action"
	TRAINID_API     = "https://xcx.xybsyw.com/student/clock/GetPlan!getDefault.action"
	POSTSIGN_API    = "https://app.xybsyw.com/behavior/Duration.action"
	SIGN_API        = "https://xcx.xybsyw.com/student/clock/Post.action"
	//HERADTOKEN_API  = "https://tenapi.cn/lab/"
	HERADTOKEN_API = "http://10.0.0.3:8000/getTokenData"
	USERIP_API     = "https://xcx.xybsyw.com/behavior/Duration!getIp.action"
)

const (
	WECHAR   = "wechar"
	PASSWORD = "password"
)

const (
	SIGNIN  = 2
	SIGNOUT = 1
)

type XybService struct {
	sessionid string
	loginerid string
	trainid   string
}

type SignData struct {
	NickName string
	Location struct {
		AdCode    string
		Longitude string
		Latitude  string
		Province  string
		Country   string
		City      string
		Address   string
	}
}

type SignMsg struct {
	Code string `json:"code"`
	Data struct {
		ClockRuleType int `json:"clockRuleType"`
		ClockInfo     struct {
			Date          string `json:"date"`
			InAddress     string `json:"inAddress"`
			InStatus      int    `json:"inStatus"`
			InStatusDesc  string `json:"inStatusDesc"`
			InTime        string `json:"inTime"`
			OutAddress    string `json:"outAddress"`
			OutStatus     int    `json:"outStatus"`
			OutStatusDesc string `json:"outStatusDesc"`
			OutTime       string `json:"outTime"`
			Status        int    `json:"status"`
			Week          string `json:"week"`
		} `json:"clockInfo"`
		CanSign               bool        `json:"canSign"`
		LocationRiskLevel     bool        `json:"locationRiskLevel"`
		TravelCodeImg         bool        `json:"travelCodeImg"`
		Explanation           interface{} `json:"explanation"`
		OpenEpidemicSituation bool        `json:"openEpidemicSituation"`
		PostInfo              struct {
			Address   string      `json:"address"`
			AddressId interface{} `json:"addressId"`
			Clock     int         `json:"clock"`
			Compare   int         `json:"compare"`
			Distance  int         `json:"distance"`
			Lat       float64     `json:"lat"`
			Lng       float64     `json:"lng"`
			State     int         `json:"state"`
		} `json:"postInfo"`
		AddressList      []interface{} `json:"addressList"`
		HealthCodeStatus bool          `json:"healthCodeStatus"`
		HealthCodeImg    bool          `json:"healthCodeImg"`
		PhysicalSymptoms interface{}   `json:"physicalSymptoms"`
		Reported         bool          `json:"reported"`
		TestResult       interface{}   `json:"testResult"`
	} `json:"data"`
	Msg string `json:"msg"`
}

type deCode struct {
	M string `json:"m"`
	S string `json:"s"`
	T int    `json:"t"`
}

func (xyb *XybService) Sign(data SignData, signType int) (err error) {

	//签到提交
	err = xyb.postSignData(data.NickName, data.Location.Country, data.Location.Province, data.Location.City)
	if err != nil {
		return err
	}

	//执行签到请求
	bodyMap := make(map[string]string)
	bodyMap["traineeId"] = xyb.trainid
	bodyMap["adcode"] = data.Location.AdCode
	bodyMap["lat"] = data.Location.Latitude
	bodyMap["lng"] = data.Location.Longitude
	bodyMap["address"] = data.Location.Address
	bodyMap["deviceName"] = "Xiaomi"
	bodyMap["punchInStatus"] = "1"
	bodyMap["clockStatus"] = fmt.Sprintf("%v", signType)

	token, err := xyb.GetHeaderToken(bodyMap)
	if err != nil {
		return err
	}

	tockenData := make(map[string]string)
	tockenData["m"] = token.M
	tockenData["s"] = token.S
	tockenData["t"] = fmt.Sprintf("%v", token.T)
	tockenData["v"] = "1.6.36"
	tockenData["n"] = "content,deviceName,keyWord,blogBody,blogTitle,getType,responsibilities,street,text,reason,searchvalue,key,answers,leaveReason,personRemark,selfAppraisal,imgUrl,wxname,deviceId,avatarTempPath,file,file,model,brand,system,deviceId,platform,code,openId,unionid"

	rep, err := xyb.httpRequest(SIGN_API, tockenData, bodyMap)
	if err != nil {
		return err
	}
	//解析JSON
	repMap := map[string]interface{}{}
	err = json.Unmarshal(rep, &repMap)
	if repMap["code"] != "200" {
		return errors.New(fmt.Sprintf("%s", repMap["msg"]))
	}
	return
}

func (xyb *XybService) Login(method string, openID, unionID string) (err error) {

	bodyMap := make(map[string]string)
	loginUrl := ""
	if method == WECHAR {
		bodyMap["openId"] = openID
		bodyMap["unionId"] = unionID
		loginUrl = WXLOGIN_API
	} else if method == PASSWORD {
		bodyMap["username"] = openID
		bodyMap["password"] = fmt.Sprintf("%x", md5.Sum([]byte(unionID)))
		loginUrl = LOGIN_API
	} else {
		return errors.New("登录方式有误")
	}
	request, err := xyb.httpRequest(loginUrl, nil, bodyMap)
	if err != nil {
		return err
	}
	//解析JSON
	repMap := map[string]interface{}{}
	err = json.Unmarshal(request, &repMap)
	if repMap["code"] != "200" {
		return errors.New(fmt.Sprintf("%s", repMap["msg"]))
	}
	//取session
	data := repMap["data"].(map[string]interface{})
	xyb.sessionid = data["sessionId"].(string)
	//取loginerid
	xyb.loginerid = fmt.Sprintf("%d", int(data["loginerId"].(float64)))
	return
}

func (xyb *XybService) GetNickName() (nickName string, err error) {

	request, err := xyb.httpRequest(UserMESSAGE_API, nil, nil)
	if err != nil {
		return "", err
	}
	//解析JSON
	repMap := map[string]interface{}{}
	json.Unmarshal(request, &repMap)
	if repMap["code"] != "200" {
		err = errors.New(fmt.Sprintf("%s", repMap["msg"]))
		return
	}
	//提取用户昵称
	data := repMap["data"].(map[string]interface{})
	nickName = data["loginer"].(string)
	return
}

func (xyb *XybService) GetSignMessage(trainId string) (SignMsg, error) {

	var msg SignMsg
	bodyMap := make(map[string]string)
	bodyMap["traineeId"] = trainId

	request, err := xyb.httpRequest(SIGNMESSAGE_API, nil, bodyMap)

	if err != nil {
		return msg, err
	}
	//解析JSON
	err = json.Unmarshal(request, &msg)
	if err != nil {
		return msg, err
	}

	if msg.Code != "200" {
		return msg, errors.New(msg.Msg)
	}
	return msg, err
}

func (xyb *XybService) GetCityCode(city string) (code string, err error) {

	bodyMap := make(map[string]string)
	bodyMap["cityName"] = city

	request, err := xyb.httpRequest(CITYCODE_API, nil, bodyMap)
	if err != nil {
		return "", err
	}
	//解析JSON
	repMap := map[string]interface{}{}
	json.Unmarshal(request, &repMap)
	if repMap["code"] != "200" {
		err = errors.New(fmt.Sprintf("%s", repMap["msg"]))
		return
	}
	//取code
	code = fmt.Sprintf("%d", int(repMap["data"].(float64)))

	return
}

func (xyb *XybService) GetTrainID() (trainId string, err error) {

	request, err := xyb.httpRequest(TRAINID_API, nil, nil)
	if err != nil {
		return "", err
	}
	//解析JSON
	repMap := map[string]interface{}{}
	json.Unmarshal(request, &repMap)
	if repMap["code"] != "200" {
		err = errors.New(fmt.Sprintf("%s", repMap["msg"]))
		return
	}
	//提取trainId
	data := repMap["data"].(map[string]interface{})
	var clockVo map[string]interface{}

	if _, ok := data["endClockVo"]; ok {
		clockVo = data["endClockVo"].(map[string]interface{})
	} else {
		clockVo = data["clockVo"].(map[string]interface{})
	}
	trainId = fmt.Sprintf("%d", int(clockVo["traineeId"].(float64)))
	xyb.trainid = trainId
	return
}

func (xyb *XybService) postSignData(nickName, country, province, city string) (err error) {

	ip, _ := xyb.getUserIP()

	bodyMap := make(map[string]string)
	bodyMap["app"] = "wx_student"
	bodyMap["appVersion"] = "1.6.36"
	bodyMap["userId"] = xyb.loginerid
	bodyMap["deviceToken"] = ""
	bodyMap["userName"] = nickName
	bodyMap["country"] = country
	bodyMap["province"] = province
	bodyMap["city"] = city
	bodyMap["deviceModel"] = "Xiaomi"
	bodyMap["operatingSystem"] = "android"
	bodyMap["operatingSystemVersion"] = "11"
	bodyMap["screenHeight"] = "800"
	bodyMap["screenWidth"] = "450"
	bodyMap["eventTime"] = fmt.Sprintf("%d", time.Now().Unix())
	bodyMap["pageId"] = "2"
	bodyMap["pageName"] = "成长"
	bodyMap["pageUrl"] = "pages/growup/growup"
	bodyMap["eventType"] = "click"
	bodyMap["eventName"] = "clickSignEvent"
	bodyMap["clientIP"] = ip
	bodyMap["reportSrc"] = "2"
	bodyMap["login"] = "1"
	bodyMap["netType"] = "WIFI"
	bodyMap["itemID"] = "none"
	bodyMap["itemType"] = "其他"

	request, err := xyb.httpRequest(POSTSIGN_API, nil, bodyMap)
	if err != nil {
		return
	}
	//解析JSON
	repMap := map[string]interface{}{}
	json.Unmarshal(request, &repMap)
	if repMap["code"] != "200" {
		err = errors.New(fmt.Sprintf("%s", repMap["msg"]))
		return
	}
	return
}

func (xyb *XybService) GetHeaderToken(body map[string]string) (code deCode, err error) {

	request, err := xyb.httpRequest(HERADTOKEN_API, nil, body)
	if err != nil {
		return code, err
	}
	err = json.Unmarshal(request, &code)
	return
}

func (xyb *XybService) getUserIP() (ip string, err error) {

	request, err := xyb.httpRequest(USERIP_API, nil, nil)

	if err != nil {
		return
	}
	//解析JSON
	repMap := map[string]interface{}{}
	json.Unmarshal(request, &repMap)
	if repMap["code"] != "200" {
		err = errors.New(fmt.Sprintf("%s", repMap["msg"]))
		return
	}
	//提取ip地址
	data := repMap["data"].(map[string]interface{})
	ip = data["ip"].(string)

	return
}

func (xyb *XybService) httpRequest(Url string, header map[string]string, body map[string]string) (rep []byte, err error) {

	client := resty.New()
	client.SetRetryCount(5).SetTimeout(time.Second * 15).SetRetryWaitTime(3 * time.Second).SetRetryMaxWaitTime(10 * time.Second)
	clientHandle := client.R()
	clientHandle.SetHeader("Content-Type", "application/x-www-form-urlencoded")

	if len(xyb.sessionid) != 0 {
		clientHandle.SetHeader("cookie", fmt.Sprintf("JSESSIONID=%s", xyb.sessionid))
	}
	//遍历添加头
	if header != nil {
		for key, val := range header {
			clientHandle.SetHeader(key, val)
		}
	}
	//遍历内容
	if body != nil {
		clientHandle.SetFormData(body)
	}

	post, err := clientHandle.Post(Url)

	if err != nil {
		return nil, err
	}
	return post.Body(), err

}
