// 处理收到的信息事件
package Processor

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/echo"
	"github.com/hoshinonyaruko/gensokyo/handlers"
	"github.com/hoshinonyaruko/gensokyo/idmap"
	"github.com/hoshinonyaruko/gensokyo/mylog"

	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/websocket/client"
)

// ProcessGroupMessage 处理群组消息
func (p *Processors) ProcessGroupMessage(data *dto.WSGroupATMessageData) error {
	// 获取s
	s := client.GetGlobalS()

	// 转换appid
	AppIDString := strconv.FormatUint(p.Settings.AppID, 10)

	// 获取当前时间的13位毫秒级时间戳
	currentTimeMillis := time.Now().UnixNano() / 1e6

	// 构造echostr，包括AppID，原始的s变量和当前时间戳
	echostr := fmt.Sprintf("%s_%d_%d", AppIDString, s, currentTimeMillis)

	var userid64 int64
	var GroupID64 int64
	var err error

	if data.Author.ID == "" {
		mylog.Printf("出现ID为空未知错误.%v\n", data)
		return nil
	}

	if !config.GetStringOb11() {
		if config.GetIdmapPro() {
			//将真实id转为int userid64
			GroupID64, userid64, err = idmap.StoreIDv2Pro(data.GroupID, data.Author.ID)
			if err != nil {
				mylog.Errorf("Error storing ID: %v", err)
			}
			//当参数不全
			_, _ = idmap.StoreIDv2(data.GroupID)
			_, _ = idmap.StoreIDv2(data.Author.ID)
			if !config.GetHashIDValue() {
				mylog.Fatalf("避坑日志:你开启了高级id转换,请设置hash_id为true,并且删除idmaps并重启")
			}
			//补救措施
			idmap.SimplifiedStoreID(data.Author.ID)
			//补救措施
			idmap.SimplifiedStoreID(data.GroupID)
			//补救措施
			echo.AddMsgIDv3(AppIDString, data.GroupID, data.ID)
		} else {
			// 映射str的GroupID到int
			GroupID64, err = idmap.StoreIDv2(data.GroupID)
			if err != nil {
				mylog.Errorf("failed to convert GroupID64 to int: %v", err)
				return nil
			}
			// 映射str的userid到int
			userid64, err = idmap.StoreIDv2(data.Author.ID)
			if err != nil {
				mylog.Printf("Error storing ID: %v", err)
				return nil
			}
		}
	}

	var messageText string
	GetDisableErrorChan := config.GetDisableErrorChan()

	//当屏蔽错误通道时候=性能模式 不解析at 不解析图片
	if !GetDisableErrorChan {
		// 转换at
		messageText = handlers.RevertTransformedText(data, "group", p.Api, p.Apiv2, GroupID64, userid64, config.GetWhiteEnable(4))
		if messageText == "" {
			mylog.Printf("信息被自定义黑白名单拦截")
			return nil
		}

		//框架内指令
		p.HandleFrameworkCommand(messageText, data, "group")
	} else {
		// 减少无用的性能开支
		messageText = data.Content

		if messageText == "/ " {
			messageText = " "
		}

		if messageText == " / " {
			messageText = " "
		}
		messageText = strings.TrimSpace(messageText)

		// 检查是否需要移除前缀
		if config.GetRemovePrefixValue() {
			// 移除消息内容中第一次出现的 "/"
			if idx := strings.Index(messageText, "/"); idx != -1 {
				messageText = messageText[:idx] + messageText[idx+1:]
			}
		}

	}

	//群没有at,但用户可以选择加一个
	if config.GetAddAtGroup() {
		messageText = "[CQ:at,qq=" + config.GetAppIDStr() + "] " + messageText
	}

	var messageID int
	//映射str的messageID到int
	if !config.GetStringOb11() {
		var messageID64 int64
		if config.GetMemoryMsgid() {
			messageID64, err = echo.StoreCacheInMemory(data.ID)
			if err != nil {
				log.Fatalf("Error storing ID: %v", err)
			}
		} else {
			messageID64, err = idmap.StoreCachev2(data.ID)
			if err != nil {
				log.Fatalf("Error storing ID: %v", err)
			}
		}
		messageID = int(messageID64)
	}

	if config.GetAutoBind() {
		if len(data.Attachments) > 0 && data.Attachments[0].URL != "" {
			p.Autobind(data)
		}
	}

	// 如果在Array模式下, 则处理Message为Segment格式
	var segmentedMessages interface{} = messageText
	if config.GetArrayValue() {
		segmentedMessages = handlers.ConvertToSegmentedMessage(data)
	}

	var IsBindedUserId, IsBindedGroupId bool
	if !config.GetStringOb11() {
		if config.GetHashIDValue() {
			IsBindedUserId = idmap.CheckValue(data.Author.ID, userid64)
			IsBindedGroupId = idmap.CheckValue(data.GroupID, GroupID64)
		} else {
			IsBindedUserId = idmap.CheckValuev2(userid64)
			IsBindedGroupId = idmap.CheckValuev2(GroupID64)
		}
	}

	var selfid64 int64
	if config.GetUseUin() {
		selfid64 = config.GetUinint64()
	} else {
		selfid64 = int64(p.Settings.AppID)
	}

	//mylog.Printf("回调测试-群:%v\n", segmentedMessages)
	var groupMsg OnebotGroupMessage
	var groupMsgS OnebotGroupMessageS
	var groupMsgMap map[string]interface{}

	// 是否使用string形式上报
	if !config.GetStringOb11() {
		groupMsg = OnebotGroupMessage{
			RawMessage:  messageText,
			Message:     segmentedMessages,
			MessageID:   messageID,
			GroupID:     GroupID64,
			MessageType: "group",
			PostType:    "message",
			SelfID:      selfid64,
			UserID:      userid64,
			Sender: Sender{
				UserID: userid64,
				Sex:    "0",
				Age:    0,
				Area:   "0",
				Level:  "0",
			},
			SubType: "normal",
			Time:    time.Now().Unix(),
		}
		//增强配置
		if !config.GetNativeOb11() {
			groupMsg.RealMessageType = "group"
			groupMsg.IsBindedUserId = IsBindedUserId
			groupMsg.IsBindedGroupId = IsBindedGroupId
			groupMsg.RealGroupID = data.GroupID
			groupMsg.RealUserID = data.Author.ID
			groupMsg.Avatar, _ = GenerateAvatarURLV2(data.Author.ID)
		}
		//根据条件判断是否增加nick和card
		var CaN = config.GetCardAndNick()
		if CaN != "" {
			groupMsg.Sender.Nickname = CaN
			groupMsg.Sender.Card = CaN
		}
		// 根据条件判断是否添加Echo字段
		if config.GetTwoWayEcho() {
			groupMsg.Echo = echostr
			//用向应用端(如果支持)发送echo,来确定客户端的send_msg对应的触发词原文
			echo.AddMsgIDv3(AppIDString, echostr, messageText)
		}
		// 获取MasterID数组
		masterIDs := config.GetMasterID()

		// 判断userid64是否在masterIDs数组里
		isMaster := false
		for _, id := range masterIDs {
			if strconv.FormatInt(userid64, 10) == id {
				isMaster = true
				break
			}
		}

		// 根据isMaster的值为groupMsg的Sender赋值role字段
		if isMaster {
			groupMsg.Sender.Role = "owner"
		} else {
			groupMsg.Sender.Role = "member"
		}
		// 将当前s和appid和message进行映射
		echo.AddMsgID(AppIDString, s, data.ID)
		echo.AddMsgType(AppIDString, s, "group")
		//为不支持双向echo的ob服务端映射
		echo.AddMsgID(AppIDString, GroupID64, data.ID)
		//将当前的userid和groupid和msgid进行一个更稳妥的映射
		echo.AddMsgIDv2(AppIDString, GroupID64, userid64, data.ID)
		//储存当前群或频道号的类型
		idmap.WriteConfigv2(fmt.Sprint(GroupID64), "type", "group")
		//映射类型
		echo.AddMsgType(AppIDString, GroupID64, "group")
		//懒message_id池
		echo.AddLazyMessageId(strconv.FormatInt(GroupID64, 10), data.ID, time.Now())
		//懒message_id池
		echo.AddLazyMessageIdv2(strconv.FormatInt(GroupID64, 10), strconv.FormatInt(userid64, 10), data.ID, time.Now())
		// 如果要使用string参数action
		if config.GetStringAction() {
			//懒message_id池
			echo.AddLazyMessageId(data.GroupID, data.ID, time.Now())
			//懒message_id池
			echo.AddLazyMessageIdv2(data.GroupID, data.Author.ID, data.ID, time.Now())
		}
		// 调试
		PrintStructWithFieldNames(groupMsg)

		// Convert OnebotGroupMessage to map and send
		groupMsgMap = structToMap(groupMsg)
	} else {
		var imgurl string
		// 自用的地方,也有一点用,有图片的时候Sender.Area是图片url(这个字段本是废弃了)
		if len(data.Attachments) > 0 {
			imgurl = data.Attachments[0].URL
		}
		groupMsgS = OnebotGroupMessageS{
			RawMessage:  messageText,
			Message:     segmentedMessages,
			MessageID:   data.ID,
			GroupID:     data.GroupID,
			MessageType: "group",
			PostType:    "message",
			SelfID:      selfid64,
			UserID:      data.Author.ID,
			Sender: Sender{
				UserID: userid64,
				Sex:    "0",
				Age:    0,
				Area:   imgurl,
				Level:  "0",
			},
			SubType: "normal",
			Time:    time.Now().Unix(),
		}
		// 增强配置
		if !config.GetNativeOb11() {
			groupMsgS.RealMessageType = "group"
			groupMsgS.RealGroupID = data.GroupID
			groupMsgS.RealUserID = data.Author.ID
			groupMsgS.Avatar, _ = GenerateAvatarURLV2(data.Author.ID)
		}
		//根据条件判断是否增加nick和card
		var CaN = config.GetCardAndNick()
		if CaN != "" {
			groupMsgS.Sender.Nickname = CaN
			groupMsgS.Sender.Card = CaN
		}
		// 根据条件判断是否添加Echo字段
		if config.GetTwoWayEcho() {
			groupMsgS.Echo = echostr
			//用向应用端(如果支持)发送echo,来确定客户端的send_msg对应的触发词原文
			echo.AddMsgIDv3(AppIDString, echostr, messageText)
		}
		// 获取MasterID数组
		masterIDs := config.GetMasterID()

		// 判断userid64是否在masterIDs数组里
		isMaster := false
		for _, id := range masterIDs {
			if strconv.FormatInt(userid64, 10) == id {
				isMaster = true
				break
			}
		}
		// 根据isMaster的值为groupMsg的Sender赋值role字段
		if isMaster {
			groupMsgS.Sender.Role = "owner"
		} else {
			groupMsgS.Sender.Role = "member"
		}
		// 将当前s和appid和message进行映射
		echo.AddMsgID(AppIDString, s, data.ID)
		echo.AddMsgType(AppIDString, s, "group")
		//储存当前群或频道号的类型
		go idmap.WriteConfigv2(data.GroupID, "type", "group")
		//懒message_id池
		echo.AddLazyMessageId(data.GroupID, data.ID, time.Now())
		//懒message_id池
		echo.AddLazyMessageIdv2(data.GroupID, data.Author.ID, data.ID, time.Now())
		// 调试
		PrintStructWithFieldNames(groupMsgS)
		// Convert OnebotGroupMessage to map and send
		groupMsgMap = structToMap(groupMsgS)
	}

	// 如果不是性能模式
	if !GetDisableErrorChan {
		//上报信息到onebotv11应用端(正反ws) 并等待返回
		go p.BroadcastMessageToAll(groupMsgMap, p.Apiv2, data)
	} else {
		// FAF式
		go p.BroadcastMessageToAllFAF(groupMsgMap, p.Apiv2, data)
	}

	return nil
}
