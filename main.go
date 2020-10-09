package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
	"io"
	"log"
	"time"
)

const headerLen = 16
const WsUrl = "wss://broadcastlv.chat.bilibili.com:443/sub"

// []byte合并操作
func BytesCombine(pBytes ...[]byte) []byte {
	return bytes.Join(pBytes, []byte(""))
}

// zlib解压数据
func ZlibUnCompress(compressSrc []byte) []byte {
	b := bytes.NewReader(compressSrc)
	var out bytes.Buffer
	r, _ := zlib.NewReader(b)
	_, _ = io.Copy(&out, r)
	return out.Bytes()
}

func GetHeader(strLen, opt uint32) []byte {
	buf := make([]byte, 16)
	switch opt {
	case 2: // 心跳
		binary.BigEndian.PutUint32(buf[0:], headerLen)         // 数据包长度
		binary.BigEndian.PutUint16(buf[4:], uint16(headerLen)) // 数据包头部长度
		binary.BigEndian.PutUint16(buf[6:], 2)                 // 协议版本
		binary.BigEndian.PutUint32(buf[8:], opt)               // 操作类型
		binary.BigEndian.PutUint32(buf[12:], 1)
		return buf
	case 7: // 握手
		binary.BigEndian.PutUint32(buf[0:], strLen+headerLen)  // 数据包长度
		binary.BigEndian.PutUint16(buf[4:], uint16(headerLen)) // 数据包头部长度
		binary.BigEndian.PutUint16(buf[6:], 2)                 // 协议版本
		binary.BigEndian.PutUint32(buf[8:], opt)               // 操作类型
		binary.BigEndian.PutUint32(buf[12:], 1)
		return buf
	default:
		return nil
	}
}

func HandleZlibMsg(buffer []byte, realLen uint32) {
	defer func() {
		err := recover()
		if err != nil {
			fmt.Println(err)
		}
	}()
	bufferLen := uint32(len(buffer)) // 获取当前tcp包长度
	// 以下部分是从tcp移植过来的，因为tcp上会有粘包的现象，所以要对长度进行判断
	if bufferLen != realLen {
		msg := string(buffer[headerLen:realLen])
		//fmt.Println(msg)
		go HandleStrMsg(msg)
		newRealLen := binary.BigEndian.Uint32(buffer[realLen : realLen+16])
		newBuffer := buffer[realLen:] // 新建一个buffer重新导入本函数
		if newRealLen > uint32(len(newBuffer)) {
			fmt.Println("不完整消息", string(newBuffer[headerLen:]))
		} else {
			HandleZlibMsg(newBuffer, newRealLen)
		}
		buffer = []byte{} // 清空buffer以被gc（不确定是否可行，刚学不知道要怎么回收内存）
	} else {
		msg := string(buffer[headerLen:realLen])
		//fmt.Println(msg)
		HandleStrMsg(msg)
	}
}

func HandleStrMsg(msg string) {
	value := gjson.Get(msg, "cmd").String()
	switch value {
	case "DANMU_MSG", "WELCOME_GUARD", "ENTRY_EFFECT", "WELCOME", "SUPER_CHAT_MESSAGE", "SUPER_CHAT_MESSAGE_JPN",
		"SUPER_CHAT_ENTRANCE", "SUPER_CHAT_MESSAGE_DELETE": // 弹幕消息、进房提示（姥爷舰长）、醒目留言
		fmt.Println("弹幕类消息", msg)
	case "INTERACT_WORD": // 进房提醒（普通用户）
		//username := gjson.Get(msg, "data.uname").String()
		//roomid := gjson.Get(msg, "data.roomid").String()
		//fmt.Println("用户",username,"进入了房间", roomid)
	case "SEND_GIFT", "COMBO_SEND", "ACTIVITY_MATCH_GIFT": // 送礼相关
		//fmt.Println("礼物类消息")
	case "ANCHOR_LOT_START":
		fmt.Println("天选开始", msg)
	case "ANCHOR_LOT_END":
		fmt.Println("天选结束", msg)
	case "ANCHOR_LOT_AWARD":
		fmt.Println("天选中奖信息", msg)
	case "ANCHOR_LOT_CHECKSTATUS":
		fmt.Println("天选检测状态", msg)
	case "GUARD_BUY": // 上舰长
		//fmt.Println("上舰长", msg)
	case "USER_TOAST_MSG":
		//fmt.Println("续费舰长", msg)
	case "SPECIAL_GIFT":
		fmt.Println("特殊礼物", msg)
	case "NOTICE_MSG": // 醒目消息（以前可以抽奖，所以单独分一个case出来）
		msgType := gjson.Get(msg, "msg_type").Uint()
		switch msgType {
		case 1, 6:
			fmt.Println("无用消息", msg)
		case 2:
			fmt.Println("醒目消息（送礼相关）", msg)
			//realRoomid := gjson.Get(msg, "real_roomid").Int()
			//checkLotteryInfo(realRoomid)
		case 3:
			fmt.Println("醒目消息（舰长相关）", msg)
			//realRoomid := gjson.Get(msg, "real_roomid").Int()
			//checkLotteryInfo(realRoomid)
		default:
			fmt.Println("msgType:", msgType, "  ", msg)
		}

	case "ACTIVITY_BANNER_UPDATE_V2", "ROOM_REAL_TIME_MESSAGE_UPDATE", "ROOM_BANNER", "PANEL",
		"ONLINERANK", "ROOM_RANK", "ROOM_CHANGE", "HOUR_RANK_AWARDS", "ROOM_BLOCK_MSG",
		"ROOM_SKIN_MSG", "GUARD_ACHIEVEMENT_ROOM", "HOT_ROOM_NOTIFY", "MATCH_TEAM_GIFT_RANK": // 排名变化、活动变化
	case "PREPARING", "VOICE_JOIN_LIST", "VOICE_JOIN_ROOM_COUNT_INFO", "LIVE": // 直播状态变化

	default:
		fmt.Println("未知消息类型", msg)
	}
}

func GetMsg(client *websocket.Conn) {
	for {
		_, message, err := client.ReadMessage()
		if err != nil {
			log.Println("GetMsg Error: ", err)
			return
		}
		Operation := message[11]      // 操作类型
		ProtocolVersion := message[7] // 协议类型
		switch Operation {
		case 3: // 心跳包回复（人气值）
			//fmt.Println("心跳包回复")
		case 5: // 普通包
			switch ProtocolVersion {
			case 0:
				msg := string(message[headerLen:])
				HandleStrMsg(msg)
				//fmt.Println(msg)
			case 2:
				newBuff := ZlibUnCompress(message[headerLen:])
				realLen := binary.BigEndian.Uint32(newBuff[:4]) // 一条消息的真实长度
				HandleZlibMsg(newBuff, realLen)
			}
		case 8: // 心跳包回复
			//msg := string(buff[16:])
			//fmt.Println(msg)
			fmt.Println("连接房间成功")
		}
	}
}

func HeartBeat(client *websocket.Conn) {
	for range time.Tick(time.Second * 30) {
		buf := GetHeader(0, 2)
		err := client.WriteMessage(websocket.BinaryMessage, buf)
		if err != nil {
			fmt.Println("发送心跳失败: ", err)
		}
	}
}

func main() {
	client, _, err := websocket.DefaultDialer.Dial(WsUrl, nil)
	if err != nil {
		log.Println("连接ws服务器失败", err)
	}
	defer client.Close()

	strBody := `{"roomid":1156962}`
	strLen := uint32(len(strBody))
	buf := GetHeader(strLen, 7)
	b := BytesCombine(buf, []byte(strBody))
	_ = client.WriteMessage(websocket.BinaryMessage, b)

	go HeartBeat(client)
	GetMsg(client)
}
