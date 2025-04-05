package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"math"
	"subinfobot/handler"
	"subinfobot/utils"
)

type Subinfo struct {
	Link           string
	AirportName    string
	ProfileWebPage string
	ExpireTime     string
	TimeRemain     string
	Upload         string
	Download       string
	Used           string
	Total          string
	Expired        int // 0:not Expired, 1:Expired, 2:unknown
	Available      int // 0:Available, 1:unavailable, 2:unknown
	DataRemain     string
}

func getSinf(link string) (error, Subinfo) {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", link, nil)
	req.Header.Add("User-Agent", "ClashforWindows/0.19.21")
	if err != nil {
		return err, Subinfo{}
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return err, Subinfo{}
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return errors.New(fmt.Sprintf("获取失败，服务器返回了代码%s", strconv.Itoa(res.StatusCode))), Subinfo{}
	}

	sinf := Subinfo{Link: link, AirportName: "未知机场", ProfileWebPage: ""}

	if cd := res.Header.Get("Content-Disposition"); cd != "" {
		parts := strings.Split(cd, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "filename=") {
				filename := strings.TrimPrefix(part, "filename=")
				filename = strings.Trim(filename, `"`)
				if dotIndex := strings.LastIndex(filename, "."); dotIndex != -1 {
					filename = filename[:dotIndex]
				}
				sinf.AirportName = filename
			} else if strings.HasPrefix(part, "filename*=") {
				filenameStar := strings.TrimPrefix(part, "filename*=")
				if strings.HasPrefix(filenameStar, "UTF-8''") {
					encoded := strings.TrimPrefix(filenameStar, "UTF-8''")
					decoded, err := url.QueryUnescape(encoded)
					if err == nil {
						if dotIndex := strings.LastIndex(decoded, "."); dotIndex != -1 {
							decoded = decoded[:dotIndex]
						}
						sinf.AirportName = decoded
					}
				}
			}
		}
	}

	if profileURL := res.Header.Get("Profile-Web-Page-Url"); profileURL != "" {
		sinf.ProfileWebPage = profileURL
	}

	if sinfo := res.Header["Subscription-Userinfo"]; sinfo == nil {
		return errors.New("💔未获取到订阅详细信息，该订阅可能已到期或已被删除"), sinf
	} else {
		sinfmap := make(map[string]int64)
		parseExp := regexp.MustCompile("[A-Za-z]+=[0-9]+")
		sslice := parseExp.FindAllStringSubmatch(sinfo[0], -1)
		for _, val := range sslice {
			kvslice := strings.Split(val[0], "=")
			if len(kvslice) == 2 {
				i, err := strconv.ParseInt(kvslice[1], 10, 64)
				if err == nil {
					sinfmap[kvslice[0]] = i
				}
			}
		}
		if upload, oku := sinfmap["upload"]; oku {
			sinf.Upload = utils.FormatFileSize(upload)
		} else {
			sinf.Upload = "没有说明捏🤔"
		}
		if download, okd := sinfmap["download"]; okd {
			sinf.Download = utils.FormatFileSize(download)
		} else {
			sinf.Download = "没有说明捏🤔"
		}
		if total, okt := sinfmap["total"]; okt {
			sinf.Total = utils.FormatFileSize(total)
			down, oka := sinfmap["download"]
			up, okb := sinfmap["upload"]
			if oka && okb {
				sinf.Used = utils.FormatFileSize(up + down)
				remain := total - (up + down)
				if remain >= 0 {
					if remain > 0 {
						sinf.Available = 0
						sinf.DataRemain = utils.FormatFileSize(remain)
					} else {
						sinf.Available = 1
						sinf.DataRemain = utils.FormatFileSize(remain)
					}
				} else {
					sinf.Available = 1
					sinf.DataRemain = "逾量" + utils.FormatFileSize(int64(math.Abs(float64(remain))))
				}
			} else {
				sinf.Available = 2
				sinf.DataRemain = "不知道捏🤔"
			}
		} else {
			sinf.Available = 2
			sinf.Total = "没有说明捏🤔"
		}
		if exp, oke := sinfmap["expire"]; oke {
			timeStamp := time.Now().Unix()
			timeExp := time.Unix(exp, 0)
			sinf.ExpireTime = timeExp.String()
			if timeStamp >= exp {
				sinf.Expired = 1
				sinf.Available = 1
				remain := timeExp.Sub(time.Now())
				if remain.Hours() > 24 {
					sinf.TimeRemain = "逾期<code>" + strconv.Itoa(int(math.Floor(remain.Hours()/24))) + "天" + strconv.Itoa(int(math.Floor(float64(int(math.Floor(remain.Hours()))%24)))) + "小时" + strconv.Itoa(int(math.Floor(float64(int(math.Floor(remain.Minutes()))%60)))) + "分" + strconv.Itoa(int(math.Floor(float64(int(math.Floor(remain.Seconds()))%60)))) + "秒" + "</code>"
				} else if remain.Minutes() > 60 {
					sinf.TimeRemain = "逾期<code>" + strconv.Itoa(int(math.Floor(float64(int(math.Floor(remain.Hours()))%24)))) + "小时" + strconv.Itoa(int(math.Floor(float64(int(math.Floor(remain.Minutes()))%60)))) + "分" + strconv.Itoa(int(math.Floor(float64(int(math.Floor(remain.Seconds()))%60)))) + "秒" + "</code>"
				} else if remain.Seconds() > 60 {
					sinf.TimeRemain = "逾期<code>" + strconv.Itoa(int(math.Floor(float64(int(math.Floor(remain.Minutes()))%60)))) + "分" + strconv.Itoa(int(math.Floor(float64(int(math.Floor(remain.Seconds()))%60)))) + "秒" + "</code>"
				} else {
					sinf.TimeRemain = "逾期<code>" + strconv.Itoa(int(math.Floor(remain.Seconds()))) + "秒" + "</code>"
				}
			} else {
				sinf.Expired = 0
				remain := timeExp.Sub(time.Now())
				if remain.Hours() > 24 {
					sinf.TimeRemain = "距离到期还有<code>" + strconv.Itoa(int(math.Floor(remain.Hours()/24))) + "天" + strconv.Itoa(int(math.Floor(float64(int(math.Floor(remain.Hours()))%24)))) + "小时" + strconv.Itoa(int(math.Floor(float64(int(math.Floor(remain.Minutes()))%60)))) + "分" + strconv.Itoa(int(math.Floor(float64(int(math.Floor(remain.Seconds()))%60)))) + "秒" + "</code>"
				} else if remain.Minutes() > 60 {
					sinf.TimeRemain = "距离到期还有<code>" + strconv.Itoa(int(math.Floor(float64(int(math.Floor(remain.Hours()))%24)))) + "小时" + strconv.Itoa(int(math.Floor(float64(int(math.Floor(remain.Minutes()))%60)))) + "分" + strconv.Itoa(int(math.Floor(float64(int(math.Floor(remain.Seconds()))%60)))) + "秒" + "</code>"
				} else if remain.Seconds() > 60 {
					sinf.TimeRemain = "距离到期还有<code>" + strconv.Itoa(int(math.Floor(float64(int(math.Floor(remain.Minutes()))%60)))) + "分" + strconv.Itoa(int(math.Floor(float64(int(math.Floor(remain.Seconds()))%60)))) + "秒" + "</code>"
				} else {
					sinf.TimeRemain = "距离到期还有<code>" + strconv.Itoa(int(math.Floor(remain.Seconds()))) + "秒" + "</code>"
				}
			}
		} else {
			sinf.ExpireTime = "♾️未知"
			sinf.TimeRemain = "可能是无限时长订阅或者服务器抽抽了呢🤣"
		}
	}
	return nil, sinf
}

func subInfoMsg(link string, update *tgbotapi.Update, bot *tgbotapi.BotAPI, msg *tgbotapi.MessageConfig) {
	msg.Text = "🕰获取中..."
	msg.ReplyToMessageID = update.Message.MessageID
	sres, err := handler.SendMsg(bot, msg)
	handler.HandleError(err)
	if err == nil {
		err, sinf := getSinf(link)
		handler.HandleError(err)
		if err != nil {
			_, err := handler.EditMsg(fmt.Sprintf("<strong>❌获取失败</strong>\n\n获取订阅<code>%s</code>时发生错误:\n<code>%s</code>", sinf.Link, err), "html", bot, sres)
			handler.HandleError(err)
			if update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup" {
				_, _ = handler.DelMsgWithTimeOut(15*24*time.Hour, bot, sres)
			}
		} else {
			var resMsg string
			if sinf.Expired == 0 && sinf.Available == 0 {
				resMsg = "✅该订阅有效"
			}
			if sinf.Expired == 2 || sinf.Available == 2 {
				resMsg = "❓该订阅状态未知"
			}
			if sinf.Expired == 1 || sinf.Available == 1 {
				resMsg = "❌该订阅不可用"
			}
			airportNameLink := sinf.AirportName
			if sinf.ProfileWebPage != "" {
				airportNameLink = fmt.Sprintf("<a href=\"%s\">%s</a>", sinf.ProfileWebPage, sinf.AirportName)
			}
			_, err = handler.EditMsg(fmt.Sprintf("<strong>%s</strong>\n🔗<strong>订阅链接:</strong><code>%s</code>\n✈️<strong>机场名称:</strong> %s\n💧<strong>总共流量:</strong><code>%s</code>\n⏳<strong>剩余流量:</strong><code>%s</code>\n⬆️<strong>已用上传:</strong><code>%s</code>\n⬇️<strong>已用下载:</strong><code>%s</code>\n⏱️<strong>该订阅将于<code>%s</code>过期,%s</strong>\n\n\n加入群组 @VPN_98K，获取更多订阅节点",
				resMsg, sinf.Link, airportNameLink, sinf.Total, sinf.DataRemain, sinf.Upload, sinf.Download, sinf.ExpireTime, sinf.TimeRemain), "html", bot, sres)
			handler.HandleError(err)
			if update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup" {
				_, _ = handler.DelMsgWithTimeOut(15*24*time.Hour, bot, sres)
			}
		}
	}
}
