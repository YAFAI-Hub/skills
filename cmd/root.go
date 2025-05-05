/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	handler "yafai-skill/handler"
	skill "yafai-skill/proto"

	"gopkg.in/yaml.v3"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func ParseAPISpec(path string) (res *handler.APISpec, err error) {
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		slog.Error("Error reading YAML file:", err)
		return nil, fmt.Errorf("error reading YAML file: %w", err) // Use fmt.Errorf for better error wrapping
	}
	slog.Info("Successfully read YAML file: %s", path)

	var apiSpec handler.APISpec
	err = yaml.Unmarshal(yamlFile, &apiSpec)
	if err != nil {
		slog.Error("Error unmarshalling YAML:", err)
		return nil, fmt.Errorf("error unmarshalling YAML: %w", err) // Use fmt.Errorf
	}
	slog.Info("Successfully unmarshalled YAML data.")
	return &apiSpec, nil
}

func StartRegisterSkill(path string, key string) error {
	slog.Info(key)

	// Get user home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	yafaiRoot := fmt.Sprintf("%s/.yafai", homeDir)
	envFile := fmt.Sprintf("%s/.env", yafaiRoot)

	// Read existing .env file
	envMap, err := godotenv.Read(envFile)
	if err != nil {
		log.Printf("Error reading %s: %v. Creating new file.", envFile, err)
		envMap = make(map[string]string)
	}

	// Update environment variables
	envMap["SKILL_KEY"] = key

	if err := godotenv.Write(envMap, envFile); err != nil {
		log.Fatalf("failed to write .env file: %v", err)
	}

	fmt.Printf("Successfully updated %s\n", envFile)

	// Load updated .env
	if err := godotenv.Load(envFile); err != nil {
		slog.Error(err.Error())
	}

	os.Setenv("SKILL_KEY", key)

	// Parse manifest
	manifest, err := ParseAPISpec(path)
	if err != nil {
		slog.Error(err.Error())
	}

	// Ensure plugins directory exists
	pluginDir := fmt.Sprintf("%s/plugins", yafaiRoot)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		log.Fatalf("failed to create plugins directory: %v", err)
	}

	// Manage socket
	sockPath := fmt.Sprintf("%s/skill.sock", pluginDir)

	// Clean old socket file
	if err := os.Remove(sockPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("failed to remove old socket file: %v", err)
	}

	// Create new UNIX socket listener
	lis, err := net.Listen("unix", sockPath)
	if err != nil {
		log.Fatalf("failed to listen on socket: %v", err)
	}
	defer lis.Close()

	// Optional: Ensure socket is cleaned up on exit
	defer func() {
		lis.Close()
		os.Remove(sockPath)
	}()

	os.Setenv("SKILL_TOKEN", key)

	s := grpc.NewServer()
	reflection.Register(s)

	srv := &handler.SkillServer{Name: "yafai-github", Description: "Github Skill for yafai", ActionsMap: manifest.Actions}
	skill.RegisterSkillServiceServer(s, srv) // Use your generated proto package

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	log.Printf("Server listening on %v", lis.Addr())
	// Create a channel to receive OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until a signal is received
	sig := <-sigChan
	log.Printf("Received signal: %v, initiating graceful shutdown...", sig)

	// Perform graceful shutdown
	s.GracefulStop()
	log.Println("Server gracefully stopped")

	return err
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "yafai-github",
	Short: "Skills Engine for YAFAI Framework",
	Long:  ``,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		path, _ := cmd.Flags().GetString("manifest")
		key, _ := cmd.Flags().GetString("skill_key")
		StartRegisterSkill(path, key)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.yafai-github.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	var manifest string
	var skill_key string
	rootCmd.PersistentFlags().StringVarP(&manifest, "manifest", "m", "", "YAFAI Skills Manifest")
	rootCmd.PersistentFlags().StringVarP(&skill_key, "skill_key", "k", "", "YAFAI Skills key")

	// Mark the "manifest" flag as required
	rootCmd.MarkPersistentFlagRequired("manifest")
	rootCmd.MarkPersistentFlagRequired("skill_key")

}
