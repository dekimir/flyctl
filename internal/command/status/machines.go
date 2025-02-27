package status

import (
	"context"
	"fmt"

	"github.com/superfly/flyctl/agent"
	"github.com/superfly/flyctl/api"
	"github.com/superfly/flyctl/client"
	"github.com/superfly/flyctl/flaps"
	"github.com/superfly/flyctl/flypg"
	"github.com/superfly/flyctl/internal/render"
	"github.com/superfly/flyctl/iostreams"
)

func renderMachineStatus(ctx context.Context, app *api.AppCompact) (err error) {
	var (
		io     = iostreams.FromContext(ctx)
		client = client.FromContext(ctx).API()
	)

	agentclient, err := agent.Establish(ctx, client)
	if err != nil {
		return fmt.Errorf("unable to establish agent: %s", err)
	}

	dialer, err := agentclient.Dialer(ctx, app.Organization.Slug)
	if err != nil {
		return fmt.Errorf("ssh: can't build tunnel for %s: %s", app.Organization.Slug, err)
	}

	flapsClient, err := flaps.New(ctx, app)
	if err != nil {
		return err
	}

	machines, err := flapsClient.ListActive(ctx)
	if err != nil {
		return err
	}

	if app.IsPostgresApp() {
		var roles = map[string]*api.Machine{}

		for _, machine := range machines {
			pgCli := flypg.NewFromInstance(fmt.Sprintf("[%s]", machine.PrivateIP), dialer)

			role, err := pgCli.NodeRole(ctx)
			if err != nil {
				// TODO - Determine best way to present this error.
				role = "error"
			}
			roles[role] = machine
		}
		return renderPGStatus(ctx, app, roles)
	}

	obj := [][]string{
		{
			app.Name,
			app.Organization.Slug,
			app.Hostname,
			app.PlatformVersion,
		},
	}

	if err = render.VerticalTable(io.Out, "App", obj, "Name", "Owner", "Hostname", "Platform"); err != nil {
		return
	}

	rows := [][]string{}

	for _, machine := range machines {
		rows = append(rows, []string{
			machine.ID,
			machine.State,
			machine.Region,
			render.MachineHealthChecksSummary(machine),
			machine.ImageRefWithVersion(),
			machine.CreatedAt,
			machine.UpdatedAt,
		})
	}

	_ = render.Table(io.Out, "", rows, "ID", "State", "Region", "Health checks", "Image", "Created", "Updated")

	return
}

func renderPGStatus(ctx context.Context, app *api.AppCompact, machines map[string]*api.Machine) (err error) {
	var io = iostreams.FromContext(ctx)

	rows := [][]string{}

	for role, machine := range machines {
		rows = append(rows, []string{
			machine.ID,
			machine.State,
			role,
			machine.Region,
			render.MachineHealthChecksSummary(machine),
			machine.ImageRefWithVersion(),
			machine.CreatedAt,
			machine.UpdatedAt,
		})
	}
	_ = render.Table(io.Out, "", rows, "ID", "State", "Role", "Region", "Health checks", "Image", "Created", "Updated")

	return
}
