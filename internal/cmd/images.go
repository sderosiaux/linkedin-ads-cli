package cmd

import (
	"fmt"

	"github.com/sderosiaux/linkedin-ads-cli/internal/api"
	"github.com/sderosiaux/linkedin-ads-cli/internal/urn"
	"github.com/spf13/cobra"
)

func newImagesCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "images",
		Short: "Upload and manage images for ad creatives",
	}
	root.AddCommand(newImagesUploadCmd())
	return root
}

func newImagesUploadCmd() *cobra.Command {
	var (
		filePath  string
		ownerID   string
		accountID string
		assetName string
	)
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload an image (PNG/JPG/GIF) for use in ads",
		Long:  "Two-step upload: initializes with LinkedIn, then PUTs the binary to the returned URL.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, cfg, err := clientFromConfig(cmd)
			if err != nil {
				return err
			}
			// --account flag or config for media library registration
			acct := accountID
			if acct == "" {
				if a, err := accountIDFromFlagOrConfig(cmd, cfg); err == nil {
					acct = a
				}
			}
			if assetName != "" && acct == "" {
				return fmt.Errorf("--name requires an account context (pass --account <id> or run 'linkedin-ads use-account <id>')")
			}
			ownerURN := urn.Wrap(urn.Organization, ownerID)

			payload := map[string]any{
				"initializeUploadRequest": map[string]any{
					"owner": ownerURN,
				},
			}
			summary := fmt.Sprintf("POST /images?action=initializeUpload + PUT binary (%s)", filePath)
			return executeWrite(cmd, summary, payload, func() error {
				res, err := api.UploadImage(cmd.Context(), c, filePath, ownerURN, acct, assetName)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", res.ImageURN)
				return err
			})
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to image file (required)")
	cmd.Flags().StringVar(&ownerID, "owner", "", "Organization id that owns the image (required)")
	cmd.Flags().StringVar(&accountID, "account", "", "Ad account id for media library registration (optional)")
	cmd.Flags().StringVar(&assetName, "name", "", "Asset name for media library (optional, requires --account)")
	_ = cmd.MarkFlagRequired("file")
	_ = cmd.MarkFlagRequired("owner")
	return cmd
}
