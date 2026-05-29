package whatsapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	waProto "go.mau.fi/whatsmeow/binary/proto"

	"github.com/lite-claw/lite-claw/internal/config"
)

// OnMessage is called for allowed inbound text messages.
type OnMessage func(ctx context.Context, sessionID, sender, text string) (reply string, err error)

// Bot manages WhatsApp connectivity.
type Bot struct {
	cfg     config.WhatsAppConfig
	dataDir string
	client  *whatsmeow.Client
	onMsg   OnMessage
	mu      sync.Mutex
}

func New(cfg config.WhatsAppConfig, dataDir string, onMsg OnMessage) *Bot {
	return &Bot{cfg: cfg, dataDir: dataDir, onMsg: onMsg}
}

// Login connects and shows QR if needed. Blocks until linked when new device.
func (b *Bot) Login(ctx context.Context) error {
	client, err := b.connect(ctx, true)
	if err != nil {
		return err
	}
	b.client = client
	return nil
}

// Run connects (or reconnects) and processes events until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	client, err := b.connect(ctx, false)
	if err != nil {
		return err
	}
	b.client = client
	client.AddEventHandler(b.handleEvent)

	<-ctx.Done()
	client.Disconnect()
	return ctx.Err()
}

func (b *Bot) connect(ctx context.Context, waitQR bool) (*whatsmeow.Client, error) {
	dbPath := fmt.Sprintf("file:%s?_foreign_keys=on", filepath.Join(b.dataDir, "whatsapp.db"))
	dbLog := waLog.Stdout("Database", "ERROR", true)
	container, err := sqlstore.New("sqlite", dbPath, dbLog)
	if err != nil {
		return nil, fmt.Errorf("sqlstore: %w", err)
	}
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		return nil, err
	}

	clientLog := waLog.Stdout("WhatsApp", "INFO", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(b.handleEvent)

	if client.Store.ID == nil {
		if !waitQR {
			return nil, fmt.Errorf("whatsapp not paired: run `lite-claw channels login`")
		}
		qrChan, _ := client.GetQRChannel(ctx)
		if err := client.Connect(); err != nil {
			return nil, err
		}
		for evt := range qrChan {
			switch evt.Event {
			case "code":
				fmt.Println("Scan this QR code with WhatsApp (Linked Devices):")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			case "success":
				fmt.Println("WhatsApp linked successfully.")
				return client, nil
			default:
				fmt.Println("Login event:", evt.Event)
			}
		}
		return nil, fmt.Errorf("qr channel closed before login")
	}

	if err := client.Connect(); err != nil {
		return nil, err
	}
	fmt.Println("WhatsApp connected.")
	return client, nil
}

func (b *Bot) handleEvent(evt interface{}) {
	switch e := evt.(type) {
	case *events.Message:
		b.onIncoming(e)
	}
}

func (b *Bot) onIncoming(e *events.Message) {
	if e.Info.IsFromMe && !b.cfg.SelfChat {
		return
	}
	text := extractText(e.Message)
	if text == "" {
		return
	}
	sender := formatJID(e.Info.Sender)
	if !b.allowed(sender) {
		return
	}

	sessionID := e.Info.Chat.String()
	ctx := context.Background()

	b.mu.Lock()
	onMsg := b.onMsg
	client := b.client
	b.mu.Unlock()

	if onMsg == nil || client == nil {
		return
	}

	reply, err := onMsg(ctx, sessionID, sender, text)
	if err != nil {
		reply = "Sorry, I hit an error: " + err.Error()
	}
	if strings.TrimSpace(reply) == "" {
		return
	}

	jid := e.Info.Chat
	_, sendErr := client.SendMessage(ctx, jid, &waProto.Message{
		Conversation: proto.String(reply),
	})
	if sendErr != nil {
		fmt.Fprintf(os.Stderr, "send message: %v\n", sendErr)
	}
}

func (b *Bot) allowed(sender string) bool {
	if len(b.cfg.AllowFrom) == 0 {
		return true
	}
	for _, a := range b.cfg.AllowFrom {
		if a == "*" {
			return true
		}
		if normalizePhone(a) == normalizePhone(sender) {
			return true
		}
	}
	return false
}

func extractText(m *waProto.Message) string {
	if m == nil {
		return ""
	}
	if c := m.GetConversation(); c != "" {
		return c
	}
	if ext := m.GetExtendedTextMessage(); ext != nil {
		return ext.GetText()
	}
	return ""
}

func formatJID(j types.JID) string {
	if j.Server == types.DefaultUserServer {
		return "+" + j.User
	}
	return j.String()
}

func normalizePhone(s string) string {
	s = strings.TrimPrefix(s, "+")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	return s
}
