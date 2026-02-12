package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: No .env file found or failed to load: %v", err)
	}

	if err != nil {
		log.Printf("Warning: No .env file found or failed to load: %v", err)
	}
	if os.Getenv("Guild_ID") == "" {
		log.Fatal("Guild_ID environment variable is required.")
	}
	if os.Getenv("Support_Topic_ID") == "" {
		log.Fatal("Support_Topic_ID environment variable is required.")
	}
}

var s *discordgo.Session

func init() { flag.Parse() }

func init() {
	var err error
	s, err = discordgo.New("Bot " + os.Getenv("Bot_Token"))
	if err != nil {
		panic("Invalid bot parameters: " + err.Error())
	}
}

var (
	integerOptionMinValue          = 1.0
	dmPermission                   = false
	defaultMemberPermissions int64 = discordgo.PermissionManageGuild

	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "open",
			Description: "Open a ticket",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "Open a ticket for the selected user",
					Required:    false,
				},
			},
		},
		{
			Name:        "close",
			Description: "Close the ticket",
		},
		{
			Name:        "add",
			Description: "Add a user to the ticket",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user to add to the ticket",
					Required:    true,
				},
			},
		},
		{
			Name:        "remove",
			Description: "Remove a user from the ticket",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "The user to remove from the ticket",
					Required:    true,
				},
			},
		},
	}
	commandHandlers = map[string]func(s *discordgo.Session, m *discordgo.InteractionCreate){
		"open": func(s *discordgo.Session, m *discordgo.InteractionCreate) {
			var user *discordgo.User
			if len(m.ApplicationCommandData().Options) > 0 {
				user = m.ApplicationCommandData().Options[0].UserValue(s)
			} else {
				user = m.Member.User
			}
			openCommand(s, m, user)
		},
		"close": func(s *discordgo.Session, m *discordgo.InteractionCreate) {
			closeCommand(s, m)
		},
		"add": func(s *discordgo.Session, m *discordgo.InteractionCreate) {
			var user *discordgo.User
			if len(m.ApplicationCommandData().Options) > 0 {
				user = m.ApplicationCommandData().Options[0].UserValue(s)
			}
			addCommand(s, m, user)
		},
		"remove": func(s *discordgo.Session, m *discordgo.InteractionCreate) {
			var user *discordgo.User
			if len(m.ApplicationCommandData().Options) > 0 {
				user = m.ApplicationCommandData().Options[0].UserValue(s)
			}
			removeCommand(s, m, user)
		},
	}
)

func openCommand(s *discordgo.Session, m *discordgo.InteractionCreate, user *discordgo.User) {

	member, err := s.GuildMember(os.Getenv("Guild_ID"), m.Member.User.ID)
	if err != nil {
		ephemeral(m, "Failed to fetch user information.")
		return
	}
	hasRole := false
	for _, role := range member.Roles {
		if role == os.Getenv("Support_Role_ID") {
			hasRole = true
			break
		}
	}
	if user.ID != m.Member.User.ID && !hasRole {
		ephemeral(m, "You do not have permission to open a ticket for another user.")
		return
	}
	channel, err := s.GuildChannelCreateComplex(os.Getenv("Guild_ID"), discordgo.GuildChannelCreateData{
		Name:     "ticket-" + user.Username,
		Type:     discordgo.ChannelTypeGuildText,
		ParentID: os.Getenv("Support_Topic_ID"),
	})
	if err != nil {
		ephemeral(m, "Failed to create a channel for the ticket.")
		return
	}

	err = s.ChannelPermissionSet(channel.ID, user.ID, discordgo.PermissionOverwriteTypeMember,
		discordgo.PermissionViewChannel|discordgo.PermissionSendMessages|discordgo.PermissionReadMessageHistory, 0)
	if err != nil {
		ephemeral(m, "Failed to add user to ticket")
		return
	}

	if os.Getenv("Support_Role_ID") != "" {
		err = s.ChannelPermissionSet(channel.ID, os.Getenv("Support_Role_ID"), discordgo.PermissionOverwriteTypeRole,
			discordgo.PermissionViewChannel|discordgo.PermissionSendMessages|discordgo.PermissionReadMessageHistory, 0)
		if err != nil {
			log.Printf("Warning: Failed to add support role to ticket: %v", err)
		}
	}

	s.ChannelMessageSend(channel.ID, "Hello "+user.Mention()+", welcome to your ticket! Please describe your issue and a staff member will assist you shortly.")

	s.InteractionRespond(m.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Created a private ticket: " + channel.Mention(),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func ephemeral(m *discordgo.InteractionCreate, text string) {
	s.InteractionRespond(m.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: text,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func closeCommand(s *discordgo.Session, m *discordgo.InteractionCreate) {
	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		ephemeral(m, "Failed to fetch the channel.")
		return
	}
	if channel.ParentID != os.Getenv("Support_Topic_ID") {
		ephemeral(m, "This command can only be used in a ticket channel.")
		return
	}
	_, err = s.ChannelDelete(channel.ID)
	if err != nil {
		panic(err)
	}
}

func addCommand(s *discordgo.Session, m *discordgo.InteractionCreate, user *discordgo.User) {
	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		ephemeral(m, "Failed to fetch the channel.")
		return
	}
	if channel.ParentID != os.Getenv("Support_Topic_ID") {
		ephemeral(m, "This command can only be used in a ticket channel.")
		return
	}
	err = s.ChannelPermissionSet(channel.ID, user.ID, discordgo.PermissionOverwriteTypeMember,
		discordgo.PermissionViewChannel|discordgo.PermissionSendMessages|discordgo.PermissionReadMessageHistory, 0)
	if err != nil {
		ephemeral(m, "Failed to add user to ticket.")
		return
	}
	ephemeral(m, "User added to ticket.")
}

func removeCommand(s *discordgo.Session, m *discordgo.InteractionCreate, user *discordgo.User) {
	if user.ID == m.Member.User.ID {
		ephemeral(m, "You cannot remove yourself from the ticket.")
		return
	}
	if user.ID == s.State.User.ID {
		ephemeral(m, "You cannot remove the bot from the ticket.")
		return
	}
	member, err := s.GuildMember(os.Getenv("Guild_ID"), user.ID)
	if err != nil {
		ephemeral(m, "Failed to fetch user information.")
		return
	}
	hasRole := false
	for _, role := range member.Roles {
		if role == os.Getenv("Support_Role_ID") {
			hasRole = true
			break
		}
	}
	if hasRole {
		ephemeral(m, "You cannot remove a staff member from the ticket.")
		return
	}
	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		ephemeral(m, "Failed to fetch the channel.")
		return
	}
	if channel.ParentID != os.Getenv("Support_Topic_ID") {
		ephemeral(m, "This command can only be used in a ticket channel.")
		return
	}
	err = s.ChannelPermissionSet(channel.ID, user.ID, discordgo.PermissionOverwriteTypeMember, 0,
		discordgo.PermissionViewChannel|discordgo.PermissionSendMessages|discordgo.PermissionReadMessageHistory)
	if err != nil {
		ephemeral(m, "Failed to remove user from ticket.")
		return
	}
	ephemeral(m, "User removed from ticket.")
}

func init() {
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
}

func main() {
	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	})
	err := s.Open()
	if err != nil {
		log.Fatalf("Cannot open the session: %v", err)
	}

	log.Println("Adding commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, v := range commands {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, os.Getenv("Guild_ID"), v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}

	defer s.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	log.Println("Press Ctrl+C to exit")
	<-stop

	if os.Getenv("RMCMD") == "true" {
		log.Println("Removing commands...")
		// // We need to fetch the commands, since deleting requires the command ID.
		// // We are doing this from the returned commands on line 375, because using
		// // this will delete all the commands, which might not be desirable, so we
		// // are deleting only the commands that we added.
		// registeredCommands, err := s.ApplicationCommands(s.State.User.ID, *os.Getenv("Guild_ID"))
		// if err != nil {
		// 	log.Fatalf("Could not fetch registered commands: %v", err)
		// }

		for _, v := range registeredCommands {
			err := s.ApplicationCommandDelete(s.State.User.ID, os.Getenv("Guild_ID"), v.ID)
			if err != nil {
				log.Panicf("Cannot delete '%v' command: %v", v.Name, err)
			}
		}
	}

	log.Println("Gracefully shutting down.")
}
