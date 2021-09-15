package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeIntoFile(t *testing.T, fileName, content string) {
	f, err := os.Create(fileName)
	require.NoError(t, err)

	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	require.NoError(t, f.Close())
}

func removeFile(t *testing.T, fileName string) {
	require.NoError(t, os.Remove(fileName))
}

func TestReadConfigFile(t *testing.T) {
	configFile := "config.yaml"
	writeIntoFile(t, configFile, `scrape_configs:
  - job_name: 'tidb'
    targets: ['0.0.0.0:10080', '0.0.0.0:10081']
`)
	defer removeFile(t, configFile)
	err := Initialize(configFile, nil)
	require.NoError(t, err)
	conf := GetGlobalConfig()
	require.Equal(t, len(conf.ScrapeConfigs), 1)
	require.Equal(t, len(conf.ScrapeConfigs[0].ProfilingConfig.PprofConfig), 6)
	require.Equal(t, conf.ScrapeConfigs[0].ScrapeInterval.String(), "1m")
	require.Equal(t, conf.ScrapeConfigs[0].ScrapeTimeout.String(), "1m")
	require.Equal(t, conf.ScrapeConfigs[0].Targets, []string{"0.0.0.0:10080", "0.0.0.0:10081"})

	// test invalid config
	writeIntoFile(t, configFile, `scrape_configs:
  - job_name: 'tidb'
    scrape_interval: 10s
    scrape_timeout: 10s
    profiling_config:
      pprof_config:
        profile:
          enabled: true
          seconds: 10`)

	err = Initialize(configFile, nil)
	require.Equal(t, err.Error(), "job tidb, profile.seconds(10) should less than the scrapscrape_timeout(10s)")
}

func TestProfileParams(t *testing.T) {
	configFile := "config.yaml"
	writeIntoFile(t, configFile, `scrape_configs:
  - job_name: 'tidb'
    scrape_interval: 10s
    scrape_timeout: 10s
    profiling_config:
      pprof_config:
        profile:
          seconds: 5
        goroutine:
          params:
            debug: 2`)
	defer removeFile(t, configFile)
	err := Initialize(configFile, nil)
	require.NoError(t, err)
	conf := GetGlobalConfig()
	require.Equal(t, len(conf.ScrapeConfigs), 1)
	require.Equal(t, len(conf.ScrapeConfigs[0].ProfilingConfig.PprofConfig), 6)
	for k, pcfg := range conf.ScrapeConfigs[0].ProfilingConfig.PprofConfig {
		require.Equal(t, *pcfg.Enabled, true)
		switch k {
		case "profile":
			require.Equal(t, pcfg.Seconds, 5)
			require.Equal(t, len(pcfg.Params), 0)
		case "goroutine":
			require.Equal(t, len(pcfg.Params), 1)
			require.Equal(t, pcfg.Params["debug"], "2")
		}
	}
}
