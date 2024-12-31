# volumedriver

# Using this plugin with Docker Swarm via Go SDK

1. Make sure the plugin is installed and enabled on all Swarm nodes:
   ```bash
   docker plugin install barealek/plexhost-driver:latest
   docker plugin enable barealek/plexhost-driver:latest
   ```
2. Use the Go Docker SDK to create a service. Below is a minimal snippet:
   ```go
   package main

   import (
       "context"
       "fmt"

       "github.com/docker/docker/api/types"
       "github.com/docker/docker/api/types/mount"
       "github.com/docker/docker/api/types/swarm"
       "github.com/docker/docker/client"
   )

   func main() {
       cli, err := client.NewClientWithOpts(client.FromEnv)
       // ...existing code...
       serviceSpec := swarm.ServiceSpec{
           // ...existing code...
           TaskTemplate: swarm.TaskSpec{
               ContainerSpec: swarm.ContainerSpec{
                   Image: "busybox",
                   Command: []string{"sleep", "3600"},
                   Mounts: []mount.Mount{
                       {
                           Type:   mount.TypeVolume,
                           Source: "my_volume",
                           Target: "/data",
                           VolumeOptions: &mount.VolumeOptions{
                               DriverConfig: &mount.Driver{
                                   Name: "barealek/plexhost-driver:latest",
                               },
                           },
                       },
                   },
               },
           },
       }

       resp, err := cli.ServiceCreate(context.Background(), serviceSpec, types.ServiceCreateOptions{})
       if err != nil {
           panic(err)
       }
       fmt.Printf("Created service: %v\n", resp.ID)
   }
   ```
