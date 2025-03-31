package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

var userStates = make(map[int64]string)
var userData = make(map[int64]map[string]float64)
var numPeople = make(map[int64]int)
var count = make(map[int64]int)
var userNames = make(map[int64][]string)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Ошибка загрузки .env файла")
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("Токен не найден в .env файле!")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true
	log.Printf("Бот запущен: %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID

		msgText := update.Message.Text

		switch userStates[chatID] {
		case "":
			handleHelp(bot, chatID)
		case "waiting for start":
			handleWaitStart(bot, chatID, msgText)
		case "waiting for count":
			handleWaitPeople(bot, chatID, msgText)
		case "waiting for each person":
			handleEachPerson(bot, chatID, msgText)
		case "waiting for amount":
			handleAmount(bot, chatID, msgText)
		case "done":
			handleDistribution(bot, chatID)
		default:
			bot.Send(tgbotapi.NewMessage(chatID, "Введите /help"))
		}
	}
}

func handleWaitStart(bot *tgbotapi.BotAPI, chatID int64, msgText string) {
	if msgText == "/start" {
		userStates[chatID] = "waiting for count"
		bot.Send(tgbotapi.NewMessage(chatID, "Введите количество человек:"))
	} else {
		bot.Send(tgbotapi.NewMessage(chatID, "Некорректный ввод! Введите /start."))
	}
}

func handleWaitPeople(bot *tgbotapi.BotAPI, chatID int64, msgText string) {
	num, err := strconv.Atoi(msgText)
	if err != nil || num <= 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "Введите корректное количество человек!"))
		return
	}

	bot.Send(tgbotapi.NewMessage(chatID, "Отлично! Теперь вводите различные имена."))

	numPeople[chatID] = num
	count[chatID] = 0
	userNames[chatID] = make([]string, num)
	userData[chatID] = make(map[string]float64)

	userStates[chatID] = "waiting for each person"
	handleEachPerson(bot, chatID, "")
}

func handleEachPerson(bot *tgbotapi.BotAPI, chatID int64, msgText string) {
	if msgText != "" {
		if count[chatID] != 0 {
			for i := 0; i < count[chatID]; i++ {
				if msgText == userNames[chatID][i] {
					bot.Send(tgbotapi.NewMessage(chatID, "Введите различные имена"))
					return
				}
			}
		}
		userNames[chatID][count[chatID]] = msgText
		count[chatID]++
	}

	if count[chatID] == numPeople[chatID] {
		bot.Send(tgbotapi.NewMessage(chatID, "Все имена введены! Теперь вводите суммы."))
		count[chatID] = 0
		userStates[chatID] = "waiting for amount"
		handleAmount(bot, chatID, "")
		return
	}

	bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Введите имя %d-го человека:", count[chatID]+1)))
}

func handleAmount(bot *tgbotapi.BotAPI, chatID int64, msgText string) {
	if msgText != "" {
		amount, err := strconv.ParseFloat(msgText, 64)
		if err != nil || amount < 0 {
			bot.Send(tgbotapi.NewMessage(chatID, "Введите корректную сумму!"))
			return
		}

		userData[chatID][userNames[chatID][count[chatID]]] = amount
		count[chatID]++
	}

	if count[chatID] < numPeople[chatID] {
		bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("Введите сумму, потраченную %s:", userNames[chatID][count[chatID]])))
	} else {
		bot.Send(tgbotapi.NewMessage(chatID, "Все суммы введены! Выполняем расчет..."))
		userStates[chatID] = "done"
		handleDistribution(bot, chatID)
	}
}

func handleHelp(bot *tgbotapi.BotAPI, chatID int64) {
	bot.Send(tgbotapi.NewMessage(chatID, "Введите /start для начала работы."))
	userStates[chatID] = "waiting for start"
}

func handleDistribution(bot *tgbotapi.BotAPI, chatID int64) {
	userStates[chatID] = ""
	UserDistribution = nil

	distribution(userData[chatID])

	if len(UserDistribution) == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "Никому ничего не нужно переводить!"))
		return
	}

	msg := "Распределение платежей:\n"
	for _, t := range UserDistribution {
		msg += fmt.Sprintf("%s должен отправить %s %.2f₽\n", t.To, t.From, t.Amount)
	}

	bot.Send(tgbotapi.NewMessage(chatID, msg))
}

type Transaction struct {
	From   string
	To     string
	Amount float64
}

var UserDistribution []Transaction

func distribution(users map[string]float64) {
	totalSum := 0.0
	numPeople := len(users)

	for _, amount := range users {
		totalSum += amount
	}
	average := totalSum / float64(numPeople)

	differences := make(map[string]float64)
	for name, amount := range users {
		differences[name] = average - amount // Меняем знак разницы
	}

	debtors := []string{}
	creditors := []string{}

	for name, diff := range differences {
		if diff > 0 {
			debtors = append(debtors, name) // Теперь должники — те, кто потратил меньше среднего
		} else if diff < 0 {
			creditors = append(creditors, name)
		}
	}

	debtorIndex := 0
	creditorIndex := 0

	for debtorIndex < len(debtors) && creditorIndex < len(creditors) {
		debtor := debtors[debtorIndex]
		creditor := creditors[creditorIndex]

		amountToTransfer := math.Min(differences[debtor], -differences[creditor]) // Обновленный расчет

		UserDistribution = append(UserDistribution, Transaction{
			From:   creditor, // Теперь те, кто потратил меньше, отправляют деньги
			To:     debtor,   // А те, кто потратил больше, получают
			Amount: amountToTransfer,
		})

		differences[debtor] -= amountToTransfer
		differences[creditor] += amountToTransfer

		if differences[debtor] <= 0 {
			debtorIndex++
		}
		if differences[creditor] >= 0 {
			creditorIndex++
		}
	}
}
