// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package runners

import (
	"math"
	"time"

	"github.com/gardener/network-problem-detector/pkg/common/config"
	"github.com/spf13/cobra"
)

type runnerArgs struct {
	args        []string
	clusterCfg  config.ClusterConfig
	config      RunnerConfig
	period      time.Duration
	scalePeriod bool
	runner      Runner
}

func (ra *runnerArgs) prepareConfig() RunnerConfig {
	config := ra.config
	if ra.period != 0 {
		config.Period = ra.period
	}
	if ra.scalePeriod && len(ra.clusterCfg.Nodes) > 1 {
		config.Period = time.Duration(math.Sqrt(float64(len(ra.clusterCfg.Nodes))) * float64(config.Period))
	}
	return config
}

func GetNewRoot(ra *runnerArgs) *cobra.Command {
	root := &cobra.Command{
		Use:   "runner",
		Short: "internal runner commands",
	}
	root.PersistentFlags().DurationVar(&ra.period, "period", 0, "overwrites default execution period")
	root.PersistentFlags().BoolVar(&ra.scalePeriod, "scale-period", false, "scales period by number of nodes")
	root.AddCommand(createPingHostCmd(ra))
	root.AddCommand(createCheckTCPPortCmd(ra))
	root.AddCommand(createCheckHTTPSGetArgs(ra))
	root.AddCommand(createDiscoverMDNSCmd(ra))
	root.AddCommand(createNSLookupCmd(ra))
	return root
}

func Parse(clusterCfg config.ClusterConfig, config RunnerConfig, args []string, shuffle bool) (Runner, error) {
	ra := &runnerArgs{}
	root := GetNewRoot(ra)

	cmd, flags, err := root.Find(args)
	if err != nil {
		return nil, err
	}

	err = cmd.ParseFlags(flags)
	if err != nil {
		return nil, cmd.FlagErrorFunc()(cmd, err)
	}

	ra.args = args
	ra.clusterCfg = clusterCfg
	if shuffle {
		ra.clusterCfg = clusterCfg.Shuffled()
	}
	ra.config = config
	ra.runner = nil
	err = cmd.RunE(cmd, flags)
	if err != nil {
		return nil, err
	}
	return ra.runner, nil
}
