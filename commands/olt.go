package commands

import (
	"context"
	"fmt"
	"goont/models"
	"goont/snmp"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v3"
)

var OltList *cli.Command = &cli.Command{
	Name:  "list",
	Usage: "listar todos los OLT",
	Action: func(ctx context.Context, c *cli.Command) error {
		store, cleanup, err := newStore(ctx)
		if err != nil {
			return err
		}
		defer cleanup()

		olts, err := store.GetInfoOLTs(ctx)
		if err != nil {
			return fmt.Errorf("Error al intentar obtener todos los OLT: %v", err)
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

		s := snmp.NewSnmp(ip, community, retries, time.Duration(timeout)*time.Second, 1)
		info, err := s.SysInfo()
		s.Close()
		if err != nil {
			return fmt.Errorf("Error al intentar realizar una consulta snmp: %v", err)
		}

		store, cleanup, err := newStore(ctx)
		if err != nil {
			return err
		}
		defer cleanup()

		err = store.InsertOLT(ctx, models.OLT{
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

		olt, err := store.GetOLTByID(ctx, ip)
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
	Flags: []cli.Flag{&cli.StringFlag{Name: "ip", Usage: "IP del olt", Required: true}},
	Action: func(ctx context.Context, c *cli.Command) error {
		ip := c.String("ip")
		if ip == "" {
			return fmt.Errorf("la IP del olt es requerida")
		}

		store, cleanup, err := newStore(ctx)
		if err != nil {
			return err
		}
		defer cleanup()

		if err := store.DeleteOLT(ctx, ip); err != nil {
			return fmt.Errorf("Error al intentar eliminar el olt: %v", err)
		}

		return nil
	},
}
