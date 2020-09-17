package cmd

import (
	"fmt"
	"github.com/NodeFactoryIo/vedran/internal/auth"
	"github.com/NodeFactoryIo/vedran/internal/router"
	"github.com/asdine/storm/v3"
	"github.com/spf13/cobra"
	"log"
	"net/http"
)

var (
	authSecretFlag string
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts vedran load balancer",
	Run:   startCommand,
}

func init() {
	startCmd.Flags().StringVar(
		&authSecretFlag,
		"auth-secret",
		"",
		"example flag")
	RootCmd.AddCommand(startCmd)
}

func startCommand(_ *cobra.Command, _ []string) {
	err := auth.SetAuthSecret(authSecretFlag)
	if err != nil {
		// terminate app: no auth secret provided
		log.Fatal(fmt.Sprintf("Unable to start vedran load balancer: %v", err))
	}

	log.Println("Starting vedran load balancer on port :4000...")

	database, err := storm.Open("my.db")
	if err != nil {
		// todo
	}

	err = http.ListenAndServe(":4000", router.CreateNewApiRouter(database))
	if err != nil {
		log.Print(err)
	}
	err = database.Close()
	log.Fatal(err)
}
