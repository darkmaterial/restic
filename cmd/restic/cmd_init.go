package main

import (
	"bytes"
	"encoding/json"

	"github.com/restic/chunker"
	"github.com/restic/restic/internal/backend/location"
	"github.com/restic/restic/internal/errors"
	"github.com/restic/restic/internal/repository"

	"github.com/spf13/cobra"
)

var cmdInit = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new repository",
	Long: `
The "init" command initializes a new repository.

EXIT STATUS
===========

Exit status is 0 if the command was successful, and non-zero if there was any error.
`,
	DisableAutoGenTag: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit(initOptions, globalOptions, args)
	},
}

// InitOptions bundles all options for the init command.
type InitOptions struct {
	secondaryRepoOptions
	CopyChunkerParameters bool
}

var initOptions InitOptions

func init() {
	cmdRoot.AddCommand(cmdInit)

	f := cmdInit.Flags()
	initSecondaryRepoOptions(f, &initOptions.secondaryRepoOptions, "secondary", "to copy chunker parameters from")
	f.BoolVar(&initOptions.CopyChunkerParameters, "copy-chunker-params", false, "copy chunker parameters from the secondary repository (useful with the copy command)")
}

func runInit(opts InitOptions, gopts GlobalOptions, args []string) error {
	chunkerPolynomial, err := maybeReadChunkerPolynomial(opts, gopts)
	if err != nil {
		return err
	}

	repo, err := ReadRepo(gopts)
	if err != nil {
		return err
	}

	be, err := create(repo, gopts.extended)
	if err != nil {
		if !gopts.JSON {
			return errors.Fatalf("create repository at %s failed: %v\n", location.StripPassword(gopts.Repo), err)
		} else {
			status := struct {
				Status     string `json:"status"`
				Repository string `json:"repository"`
				Message    string `json:"message"`
			}{
				Status:     "error_repository",
				Repository: location.StripPassword(gopts.Repo),
				Message:    err.Error(),
			}
			return errors.Fatalf(toJSONString(status))
		}
	}

	gopts.password, err = ReadPasswordTwice(gopts,
		"enter password for new repository: ",
		"enter password again: ")
	if err != nil {
		return err
	}

	s := repository.New(be)

	err = s.Init(gopts.ctx, gopts.password, chunkerPolynomial)
	if err != nil {
		if !gopts.JSON {
			return errors.Fatalf("create key in repository at %s failed: %v\n", location.StripPassword(gopts.Repo), err)
		} else {
			status := struct {
				Status     string `json:"status"`
				Repository string `json:"repository"`
				Message    string `json:"message"`
			}{
				Status:     "error_key",
				Repository: location.StripPassword(gopts.Repo),
				Message:    err.Error(),
			}
			return errors.Fatalf(toJSONString(status))
		}
	}

	if !gopts.JSON {
		Verbosef("created restic repository %v at %s\n", s.Config().ID[:10], location.StripPassword(gopts.Repo))
		Verbosef("\n")
		Verbosef("Please note that knowledge of your password is required to access\n")
		Verbosef("the repository. Losing your password means that your data is\n")
		Verbosef("irrecoverably lost.\n")

	} else {
		status := struct {
			Status     string `json:"status"`
			Id         string `json:"id"`
			Repository string `json:"repository"`
		}{
			Status:     "success",
			Id:         s.Config().ID[:10],
			Repository: location.StripPassword(gopts.Repo),
		}
		Verbosef(toJSONString(status))
	}

	return nil
}

func toJSONString(status interface{}) string {
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(status)
	return buf.String()
}

func maybeReadChunkerPolynomial(opts InitOptions, gopts GlobalOptions) (*chunker.Pol, error) {
	if opts.CopyChunkerParameters {
		otherGopts, err := fillSecondaryGlobalOpts(opts.secondaryRepoOptions, gopts, "secondary")
		if err != nil {
			return nil, err
		}

		otherRepo, err := OpenRepository(otherGopts)
		if err != nil {
			return nil, err
		}

		pol := otherRepo.Config().ChunkerPolynomial
		return &pol, nil
	}

	if opts.Repo != "" {
		return nil, errors.Fatal("Secondary repository must only be specified when copying the chunker parameters")
	}
	return nil, nil
}
