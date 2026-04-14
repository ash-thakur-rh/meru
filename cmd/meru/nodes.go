package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var nodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "Manage remote meru-node targets",
}

// --- nodes add ---

var (
	nodeAddr  string
	nodeToken string
	nodeTLS   bool
)

var nodesAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Register a remote meru-node",
	Example: `  meru nodes add gpu-box --addr 10.0.0.5:9090 --token mysecret
  meru nodes add cloud-vm --addr vm.example.com:9090 --token s3cr3t --tls`,
	Args: cobra.ExactArgs(1),
	RunE: runNodesAdd,
}

func init() {
	nodesAddCmd.Flags().StringVar(&nodeAddr, "addr", "", "gRPC address of the remote node (host:port) (required)")
	nodesAddCmd.Flags().StringVar(&nodeToken, "token", "", "Bearer token configured on the remote node (required)")
	nodesAddCmd.Flags().BoolVar(&nodeTLS, "tls", false, "Use TLS when connecting")
	nodesAddCmd.MarkFlagRequired("addr")  //nolint:errcheck
	nodesAddCmd.MarkFlagRequired("token") //nolint:errcheck
	nodesCmd.AddCommand(nodesAddCmd)
}

func runNodesAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	var result map[string]any
	err := doJSON("POST", "/nodes", map[string]any{
		"name":  name,
		"addr":  nodeAddr,
		"token": nodeToken,
		"tls":   nodeTLS,
	}, &result)
	if err != nil {
		return err
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
	return nil
}

// --- nodes list ---

var nodesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered nodes",
	RunE:  runNodesList,
}

func init() {
	nodesCmd.AddCommand(nodesListCmd)
}

func runNodesList(cmd *cobra.Command, args []string) error {
	var nodes []map[string]any
	if err := doJSON("GET", "/nodes", nil, &nodes); err != nil {
		return err
	}
	if len(nodes) == 0 {
		fmt.Println("No nodes registered. Use: meru nodes add <name> --addr host:port --token secret")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tADDR\tTLS\tAGENTS\tLAST SEEN")
	for _, n := range nodes {
		tls := "no"
		if b, ok := n["tls"].(bool); ok && b {
			tls = "yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			str(n["name"]),
			str(n["addr"]),
			tls,
			str(n["agents"]),
			str(n["last_seen"]),
		)
	}
	return w.Flush()
}

// --- nodes remove ---

var nodesRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Unregister a remote node",
	Args:  cobra.ExactArgs(1),
	RunE:  runNodesRemove,
}

func init() {
	nodesCmd.AddCommand(nodesRemoveCmd)
}

func runNodesRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := doJSON("DELETE", "/nodes/"+name, nil, nil); err != nil {
		return err
	}
	fmt.Printf("Node %q removed.\n", name)
	return nil
}

// --- nodes ping ---

var nodesPingCmd = &cobra.Command{
	Use:   "ping <name>",
	Short: "Ping a node to check connectivity and capabilities",
	Args:  cobra.ExactArgs(1),
	RunE:  runNodesPing,
}

func init() {
	nodesCmd.AddCommand(nodesPingCmd)
	rootCmd.AddCommand(nodesCmd)
}

func runNodesPing(cmd *cobra.Command, args []string) error {
	name := args[0]
	var result map[string]any
	if err := doJSON("POST", "/nodes/"+name+"/ping", nil, &result); err != nil {
		return err
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
	return nil
}
