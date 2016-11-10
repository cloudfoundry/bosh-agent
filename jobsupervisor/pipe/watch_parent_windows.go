package main

import (
	"log"
	"os"

	"github.com/greenhouse-org/ps"
)

func watchParent(exitMain func(int)) error {
	jobObjectName := pseudo_uuid()
	jobObject, err := ps.CreateJobObject(jobObjectName)
	if err != nil {
		log.Printf("Failed to create JobObject: %v\n", err)
		return err
	}
	err = jobObject.AddCurrentProcess()
	if err != nil {
		log.Printf("Failed to add process to JobObject: %v\n", err)
		return err
	}
	parent, err := os.FindProcess(os.Getppid())
	if err != nil {
		log.Printf("Failed to find parent process : %v\n", err)
		return err
	}
	go func() {
		_, err := parent.Wait()
		if err != nil {
			log.Printf("Failed to wait for parent process : %v\n", err)
			return
		}
		log.Println("Parent process exited, we will exit")
		exitMain(1)
		jobObject.Terminate(1)
	}()

	return nil
}
