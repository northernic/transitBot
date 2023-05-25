package main

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"
)

var (
	Conf       = Config{}
	LOG        = "logrus.log"
	log        *logrus.Logger
	configName = "config.yaml"
	bot        *tgbotapi.BotAPI
	userStates map[int64]*UserState
)

type UserState struct {
	Uid               int
	LastCallbackMsgID int
	LastCallbackData  string
	ErrorCode         string
	//Sign              bool //true代表已处理
}

func initConfig() {
	files, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		fmt.Println("读取配置失败,err: ", err.Error())
	}
	err = yaml.Unmarshal(files, &Conf)
	if err != nil {
		fmt.Println("读取配置失败,err: ", err.Error())
	}
	//初始化log
	log = logrus.New()
	file, err := os.OpenFile(LOG, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		log.Error("Failed to open log file: ", err)
	} else {
		log.SetOutput(file)
	}
	fmt.Println("读取配置成功")
}

func initBot() {
	var err error
	bot, err = tgbotapi.NewBotAPI(Conf.BotToken)
	if err != nil {
		log.Error("bot创建出错，错误信息： " + err.Error())
	}
	bot.Debug = true
	log.Printf("Authorized on account: %s  ID: %d", bot.Self.UserName, bot.Self.ID)
	userStates = make(map[int64]*UserState)
}

func main() {
	initConfig()
	initBot()
	go startBot()
	select {}
}

// 发送消息给指定聊天ID
func sendMsg(chatID int64, msg string) {
	if msg == "" {
		return
	}
	tgMsg := tgbotapi.NewMessage(chatID, msg)
	_, err := bot.Send(tgMsg)
	if err != nil {
		log.Error("bot发送信息出错，错误信息： " + err.Error())
	}
}

// 读取 config.yaml 文件并返回 Config 结构体
func readConfigFile() (*Config, error) {
	config := &Config{}

	content, err := ioutil.ReadFile(configName)
	if err != nil {
		fmt.Println("读取配置失败,err: ", err.Error())
	}

	err = yaml.Unmarshal(content, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func getFieldInfo(value reflect.Value) string {
	typeName := value.Type()
	var st []string
	for i := 0; i < value.NumField(); i++ {
		typeField := typeName.Field(i)
		fieldName := typeField.Name
		fieldValue := value.Field(i).Interface()

		// 处理切片类型
		if value.Field(i).Kind() == reflect.Slice {
			sliceValues := make([]string, value.Field(i).Len())
			for j := 0; j < value.Field(i).Len(); j++ {
				sliceValues[j] = fmt.Sprintf("%v", value.Field(i).Index(j))
			}
			fieldValue = strings.Join(sliceValues, "\n")
		}
		//仅展示配置项
		if fieldValue != "" {
			tmpSt := fmt.Sprintf("%s:\n%v\n", fieldName, fieldValue)
			st = append(st, tmpSt)
		}
	}
	return strings.Join(st, "\n")

}

func startBot() {
	// 设置机器人接收更新的方式
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, _ := bot.GetUpdatesChan(u)
	// 处理接收到的更新
	for update := range updates {
		if update.CallbackQuery != nil {
			//处理回调
			handleCallback(update.CallbackQuery)
		} else if update.Message != nil {
			if update.Message.IsCommand() {
				//处理命令
				handleCmd(update.Message)
			} else {
				//处理普通消息
				handleMessage(update.Message)
			}
		}

		//记录请求
		//log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
		//botName := bot.Self.UserName
		//update.Message.Text = strings.ReplaceAll(update.Message.Text, "@"+botName, "")
		//update.Message.Text = strings.TrimSpace(update.Message.Text)

	}
}

// 仅开头为"/"才处理
// 单重命令(英文)，示例  /hello
func handleCmd(message *tgbotapi.Message) {
	cmd := strings.ToLower(message.Command())
	switch cmd {
	case "hello":
		sendMsg(message.Chat.ID, "hello,world!")
	case "groupid":
		sendMsg(message.Chat.ID, "groupID: "+strconv.Itoa(int(message.Chat.ID)))
	case "myid":
		sendMsg(message.Chat.ID, "myID: "+strconv.Itoa(message.From.ID))

	case "start":
		reply := "欢迎使用机器人！请从下面的选项中选择一个操作："
		// 创建内联键盘
		inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("错误上报", "错误上报"),
				tgbotapi.NewInlineKeyboardButtonData("错误处理确认", "错误处理确认"),
			),
			//tgbotapi.NewInlineKeyboardRow(
			//	tgbotapi.NewInlineKeyboardButtonData("选项3", "option3"),
			//	tgbotapi.NewInlineKeyboardButtonData("选项4", "option4"),
			//),
		)
		// 将键盘添加到回复消息中
		msg := tgbotapi.NewMessage(message.Chat.ID, reply)
		msg.ReplyMarkup = inlineKeyboard

		_, err := bot.Send(msg)
		if err != nil {
			log.Println(err)
		}

	default:
		//cmdlist := []string{
		//	"命令列表大全:",
		//	"/hello",
		//	"/groupID",
		//	"/myid",
		//	"/start",
		//	"/错误上报/{错误域名}",
		//	"/错误已处理/{群名称}/{域名}",
		//}
		//text := strings.Join(cmdlist, "\n")
		//sendMsg(message.Chat.ID, text)
	}
}

func handleMessage(message *tgbotapi.Message) {

	chatID := message.Chat.ID
	// 获取用户状态
	state, ok := userStates[chatID]

	if ok {
		diff := message.MessageID - state.LastCallbackMsgID
		if diff > 0 && diff < 6 && message.From.ID == state.Uid {
		}
		//处理对内联键盘回复的消息
		fromGroups := Conf.FromGroups
		for k, v := range fromGroups {
			if v == message.Chat.ID {
				//给本群
				sendMsg(v, "错误已提交")
				//给错误接受群
				sendMsg(Conf.HandleGroups["域名处理群"], "错误码："+state.ErrorCode+" 错误信息: "+message.Text+"来自："+k)
				// 清除用户状态
				delete(userStates, chatID)
				break
			}
		}
		return

	}
	if message.Text == "" {
		return
	}
	// 处理用户的文本输入，可以根据需要进行逻辑处理
	//reply := "收到您的输入：" + message.Text
	//
	//msg := tgbotapi.NewMessage(message.Chat.ID, reply)
	//_, err := bot.Send(msg)
	//if err != nil {
	//	log.Println(err)
	//}

}

func handleCallback(callback *tgbotapi.CallbackQuery) {
	//用户ID
	chatID := callback.Message.Chat.ID

	//用户当前状态
	state, ok := userStates[chatID]
	if !ok {
		state = &UserState{}
		userStates[chatID] = state
	}
	//保存回调状态信息
	state.LastCallbackMsgID = callback.Message.MessageID
	state.LastCallbackData = callback.Data
	state.Uid = callback.From.ID

	switch callback.Data {
	case "错误上报":
		// 生成选项一的下一层内联键盘
		nextLevelInlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("601", "601"), // 错误码后续在这里更新，并增加case的处理
			),
		)
		// 更新原始消息的内联键盘为下一层内联键盘
		//editMsg := tgbotapi.NewEditMessageReplyMarkup(callback.Message.Chat.ID, callback.Message.MessageID, nextLevelInlineKeyboard)
		//_, err := bot.Send(editMsg)

		reply := "选择错误码："
		editMsgText := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, reply)
		editMsgText.ReplyMarkup = &nextLevelInlineKeyboard // 设置新的内联键盘
		_, err := bot.Send(editMsgText)
		if err != nil {
			log.Println(err)
		}

	case "错误处理确认":
		// 错误已处理回复
		// 生成选项一的下一层内联键盘
		nextLevelInlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				//tgbotapi.NewInlineKeyboardButtonData("盘口", "盘口"),
				tgbotapi.NewInlineKeyboardButtonData("test1", "test1"),
				tgbotapi.NewInlineKeyboardButtonData("1群", "1群"),
				tgbotapi.NewInlineKeyboardButtonData("2群", "1群"),
				tgbotapi.NewInlineKeyboardButtonData("3群", "3群"),
			),
		)
		reply := "选择群："
		editMsgText := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, reply)
		editMsgText.ReplyMarkup = &nextLevelInlineKeyboard // 设置新的内联键盘
		_, err := bot.Send(editMsgText)
		if err != nil {
			log.Println(err)
		}
	case "601":
		state.ErrorCode = "601"
		reply := "输入错误域名："
		sendMsg(callback.Message.Chat.ID, reply)
	case "test1":
		//处理对内联键盘回复的消息
		fromGroups := Conf.FromGroups
		for k, _ := range fromGroups {
			if k == "test1" {
				//给本群
				sendMsg(callback.Message.Chat.ID, "已通知")
				//给错误接受群
				sendMsg(Conf.FromGroups["test1"], "错误已处理")
				break
			}
		}
	case "1群":
		//处理对内联键盘回复的消息
		fromGroups := Conf.FromGroups
		for k, _ := range fromGroups {
			if k == "test1" {
				//给本群
				sendMsg(callback.Message.Chat.ID, "已通知")
				//给错误接受群
				sendMsg(Conf.FromGroups["test1"], "错误已处理")
				break
			}
		}
	default:
		// 处理未知的回调查询数据
	}
}

// 每个update单独处理
//func processUpdate(update *tgbotapi.Update) {
//	var msg tgbotapi.MessageConfig
//	upmsg := update.Message
//	gid := upmsg.Chat.ID
//	uid := upmsg.From.ID
//	replyText := findKey(gid, upmsg.Text)
//	if replyText == "delete" {
//		_, _ = bot.DeleteMessage(api.NewDeleteMessage(gid, upmsg.MessageID))
//	} else if strings.HasPrefix(replyText, "ban") {
//		_, _ = bot.DeleteMessage(api.NewDeleteMessage(gid, upmsg.MessageID))
//		banMember(gid, uid, -1)
//	} else if strings.HasPrefix(replyText, "kick") {
//		_, _ = bot.DeleteMessage(api.NewDeleteMessage(gid, upmsg.MessageID))
//		kickMember(gid, uid)
//	} else if strings.HasPrefix(replyText, "photo:") {
//		sendPhoto(gid, replyText[6:])
//	} else if strings.HasPrefix(replyText, "gif:") {
//		sendGif(gid, replyText[4:])
//	} else if strings.HasPrefix(replyText, "video:") {
//		sendVideo(gid, replyText[6:])
//	} else if strings.HasPrefix(replyText, "file:") {
//		sendFile(gid, replyText[5:])
//	} else if replyText != "" {
//		msg = api.NewMessage(gid, replyText)
//		msg.DisableWebPagePreview = true
//		msg.ReplyToMessageID = upmsg.MessageID
//		sendMessage(msg)
//	}
//}
