package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/cert-injector/command"
	"code.cloudfoundry.org/cert-injector/container"
	"code.cloudfoundry.org/filelock"
)

const (
	LockFileName    = "GrootRootfsMutex"
	grootBin        = "c:\\var\\vcap\\packages\\groot\\groot.exe"
	wincBin         = "c:\\var\\vcap\\packages\\winc\\winc.exe"
	diffExporterBin = "c:\\var\\vcap\\packages\\diff-exporter\\diff-exporter.exe"
	hydrateBin      = "c:\\var\\vcap\\packages\\hydrate\\hydrate.exe"
)

type cmd interface {
	Run(executable string, args ...string) ([]byte, []byte, error)
}

type conf interface {
	Write(bundleDir string, certData []byte) error
}

func Run(args []string, cmd cmd, conf conf) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: %s <driver_store> <cert_file> <image_uri>...\n", args[0])
	}

	certFile := args[2]
	certData, err := ioutil.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("Failed to read cert_file: %s", err)
	}

	if len(certData) == 0 {
		return nil
	}

	ociImageUris := args[3:]
	for _, ociImageUri := range ociImageUris {
		_, _, err = cmd.Run(hydrateBin, "remove-layer", "-ociImage", ociImageUri)
		if err != nil {
			return fmt.Errorf("hydrate.exe remove-layer failed: %s\n", err)
		}
	}

	grootDriverStore := args[1]

	// TODO: the hydrator that is altering the rootfs is not looking at this lock file anyway, so we are punting right now
	lock, err := filelock.NewLocker(filepath.Join(os.TempDir(), LockFileName)).Open()
	if err != nil {
		return fmt.Errorf("open lock: %s\n", err)
	}
	defer lock.Close()

	// workaround for https://github.com/Microsoft/hcsshim/issues/155
	// fmt.Printf("%s\n", "Deleting existing containers")
	// _, _, err = cmd.Run("powershell.exe", "-c", fmt.Sprintf("Get-ComputeProcess | foreach { & %s delete $_.Id }", wincBin))
	// if err != nil {
	// 	return fmt.Errorf("Cannot delete existing containers\n")
	// }
	//
	// files, err := ioutil.ReadDir(fmt.Sprintf("%s\\volumes", grootDriverStore))
	// if !os.IsNotExist(err) {
	// 	return fmt.Errorf("groot delete failed: %s\n", err)
	// }
	//
	// for _, file := range files {
	// 	_, _, err = cmd.Run(grootBin, "--driver-store", grootDriverStore, "delete", file.Name())
	// 	if err != nil {
	// 		return fmt.Errorf("groot delete failed: %s", err)
	// 	}
	// }
	// fmt.Printf("%s\n", "Begin exporting layer")
	// for _, uri := range ociImageUris {
	//
	// var grootOutput []byte
	_, _, err = cmd.Run(grootBin, "--driver-store", grootDriverStore, "create", ociImageUris[0])
	if err != nil {
		return fmt.Errorf("groot create failed: %s", err)
	}
	//
	// 	var config map[string]interface{}
	// 	if err := json.Unmarshal(grootOutput, &config); err != nil {
	// 		return fmt.Errorf("failed to parse process spec\n")
	// 	}
	//

	containerId := fmt.Sprintf("layer-%d", int32(time.Now().Unix()))
	bundleDir := filepath.Join(os.TempDir(), containerId)
	if err = os.Mkdir(bundleDir, 0755); err != nil {
		return fmt.Errorf("Failed to create bundle directory: %s\n", err)
	}

	err = conf.Write(bundleDir, certData)
	if err != nil {
		return fmt.Errorf("Write container config failed: %s", err)
	}

	//
	// 	//TODO: merge the config.json previously created, with the output of groot and write it out as config.json
	// 	configFile.Sync()
	//

	_, _, err = cmd.Run(wincBin, "run", "-b", bundleDir, containerId)
	if err != nil {
		return fmt.Errorf("winc run failed: %s", err)
	}
	//
	// 	fmt.Printf("%s\n", "Running diff-exporter")
	diffOutputFile := filepath.Join(os.TempDir(), fmt.Sprintf("diff-output%d", int32(time.Now().Unix())))
	_, _, err = cmd.Run(diffExporterBin, "-outputFile", diffOutputFile, "-containerId", containerId, "-bundlePath", bundleDir)
	if err != nil {
		return fmt.Errorf("diff-exporter failed: %s", err)
	}
	//
	// 	fmt.Printf("%s\n", "Running hydrator")
	// 	_, _, err = cmd.Run(hydrateBin, "add-layer", "-ociImage", uri, "-layer", diffOutputFile)
	// 	if err != nil {
	// 		return fmt.Errorf("hydrator failed\n")
	// 	}
	//
	// 	fmt.Printf("%s\n", "Cleaning up")
	// 	_, _, err = cmd.Run(grootBin, "--driver-store", grootDriverStore, "delete", containerId)
	// 	if err != nil {
	// 		return fmt.Errorf("groot delete failed\n")
	// 	}
	// 	err = os.RemoveAll(diffOutputFile)
	// 	if err != nil {
	// 		return fmt.Errorf("diff output file deletion failed\n")
	// 	}
	// }

	if err = os.Mkdir(bundleDir, 0755); err != nil {
		return fmt.Errorf("Failed to create bundle directory: %s\n", err)
	}

	return nil
}

func main() {
	logger := log.New(os.Stderr, "", 0)
	cmd := command.NewCmd()
	conf := container.NewConfig()
	if err := Run(os.Args, cmd, conf); err != nil {
		logger.Print(err)
		os.Exit(1)
	}
}
