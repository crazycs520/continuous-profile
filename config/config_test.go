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
	writeIntoFile(t, configFile, `pd_address: '0.0.0.0:2379'`)
	defer removeFile(t, configFile)
	err := Initialize(configFile, nil)
	require.NoError(t, err)
	conf := GetGlobalConfig()
	require.Equal(t, conf.PDAddr, "0.0.0.0:2379")

	// test invalid config
	writeIntoFile(t, configFile, `scrape_configs:
  - component_name: 'tidb'
    scrape_interval: 10s
    scrape_timeout: 10s
    profiling_config:
      pprof_config:
        profile:
          enabled: true
          seconds: 10
    targets: ['0.0.0.0:10080', '0.0.0.0:10081']`)

	err = Initialize(configFile, nil)
	require.Equal(t, err.Error(), "job tidb, profile.seconds(10) should less than the scrapscrape_timeout(10s)")
}
