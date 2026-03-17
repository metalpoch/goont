package commands

import (
	"context"
	"fmt"
	"goont/models"
	"goont/snmp"
	"goont/storage"
	"os"
	"path/filepath"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v3"
)

var OltList *cli.Command = &cli.Command{
	Name:  "list",
	Usage: "listar todos los OLT",
	Action: func(ctx context.Context, c *cli.Command) error {
		olts, err := getAllInfoOLTS()
		if err != nil {
			return err
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.Header([]string{"IP", "Community", "Acronimo", "Ubicación", "Creado", "Actualizado"})
		table.Bulk(olts)
		table.Render()
		return nil
	},
}

var OltAdd *cli.Command = &cli.Command{
	Name:  "add",
	Usage: "agregar un nuevo OLT",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "ip", Usage: "OLT IP", Required: true},
		&cli.StringFlag{Name: "community", Usage: "SNMP Community V2", Required: true},
		&cli.IntFlag{Name: "timeout", Usage: "SNMP Timeout (segundos)", Value: 60},
		&cli.IntFlag{Name: "retries", Usage: "Reintentos en consultas SNMP", Value: 3},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		timeout, retries := c.Int("timeout"), c.Int("retries")
		ip, community := c.String("ip"), c.String("community")

		if ip == "" {
			return fmt.Errorf("la IP del olt es requerida")
		}
		if community == "" {
			return fmt.Errorf("la comunidad del olt es requerida")
		}

		s := snmp.NewSnmp(ip, community, retries, time.Duration(timeout)*time.Second)
		info, err := s.SysInfo()
		if err != nil {
			return fmt.Errorf("Error al intentar realizar una consulta snmp: %v", err)
		}

		dir, err := os.Executable()
		if err != nil {
			return fmt.Errorf("Error al intentar obtener la ruta del ejecutable")
		}

		dbDir := filepath.Join(filepath.Dir(dir), "olt.db")
		client, err := storage.NewOltDB(dbDir)
		if err != nil {
			return fmt.Errorf("Error al intentar acceder a la base de datos: %v", err)
		}

		defer client.Close()

		err = client.InsertOLT(models.OLT{
			IP:        ip,
			Community: community,
			Name:      info.SysName,
			Location:  info.SysLocation,
			Timeout:   timeout,
			Retries:   retries,
		})

		if err != nil {
			return fmt.Errorf("Error intentando ingresar el OLT: %v", err)
		}

		olt, err := client.GetOLTByID(ip)
		if err != nil {
			return fmt.Errorf("OLT agregado con error, no se pudo recuperar los datos almacenados: %v", err)
		}

		fmt.Printf("El olt %s se ha registrado correctamente\n", olt.Name)
		return nil
	},
}

var OltRemove *cli.Command = &cli.Command{
	Name:  "remove",
	Usage: "eliminar OLT",
	Flags: []cli.Flag{&cli.IntFlag{Name: "ip", Usage: "IP del olt", Required: true}},
	Action: func(ctx context.Context, c *cli.Command) error {
		dir, err := os.Executable()
		if err != nil {
			return fmt.Errorf("Error al intentar obtener la ruta del ejecutable")
		}

		dbDir := filepath.Join(filepath.Dir(dir), "olt.db")
		client, err := storage.NewOltDB(dbDir)
		if err != nil {
			return fmt.Errorf("Error al intentar acceder a la base de datos: %v", err)
		}

		defer client.Close()

		if err := client.DeleteOLT(c.String("ip")); err != nil {
			return fmt.Errorf("Error al intentar eliminar el olt: %v", err)
		}

		return nil
	},
}
