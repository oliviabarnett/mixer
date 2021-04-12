package mixer

import (
	"github.com/spf13/cobra"
	"os"
)

var (
	mainCmd = &cobra.Command{
		Use:           os.Args[0],
		Short:         "Run a JobCoin Mixer",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
)

func init() {
	mainCmd.AddCommand(
		ClientCmd,
		//ServerCmd,
	)
}

func main() {
	if c, err := mainCmd.ExecuteC(); err != nil {
		c.Println("Error:", err)
		os.Exit(-1)
	}
}