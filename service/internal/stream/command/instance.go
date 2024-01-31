package command

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lhjnilsson/foreverbull/internal/postgres"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/service/container"
	"github.com/lhjnilsson/foreverbull/service/internal/repository"
	"github.com/lhjnilsson/foreverbull/service/internal/stream/dependency"
	st "github.com/lhjnilsson/foreverbull/service/stream"
	"golang.org/x/sync/errgroup"
)

func InstanceInterview(ctx context.Context, message stream.Message) error {
	db := message.MustGet(stream.DBDep).(postgres.Query)

	var instance st.InstanceInterviewCommand
	err := message.ParsePayload(&instance)
	if err != nil {
		return fmt.Errorf("error unmarshalling InstanceInterview payload: %w", err)
	}

	services := repository.Service{Conn: db}
	instances := repository.Instance{Conn: db}

	i, err := instances.Get(ctx, instance.ID)
	if err != nil {
		return fmt.Errorf("error getting instance: %w", err)
	}
	info, err := i.GetInfo()
	if err != nil {
		return fmt.Errorf("error reading instance info: %w", err)
	}
	err = services.UpdateServiceInfo(ctx, i.Service, *info.Type, info.WorkerParameters)
	if err != nil {
		return fmt.Errorf("error updating service info: %w", err)
	}
	return nil
}

func InstanceSanityCheck(ctx context.Context, message stream.Message) error {
	db := message.MustGet(stream.DBDep).(postgres.Query)

	var instance st.InstanceSanityCheckCommand
	err := message.ParsePayload(&instance)
	if err != nil {
		return fmt.Errorf("error unmarshalling InstanceSanityCheck payload: %w", err)
	}

	instances := repository.Instance{Conn: db}
	checkOnline := func(ctx context.Context, id string) error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				i, err := instances.Get(ctx, id)
				if err != nil {
					var pgError *pgconn.PgError
					if errors.As(err, &pgError); pgError.Code == "02000" {
						time.Sleep(time.Second / 5)
						continue
					}
					return fmt.Errorf("error getting instance: %w", err)
				}
				if i.Host == nil || i.Port == nil {
					time.Sleep(time.Second / 5)
					continue
				}
				info, err := i.GetInfo()
				if err != nil {
					return fmt.Errorf("error reading instance info: %w", err)
				}
				if i.ServiceType != nil {
					if strings.Compare(*i.ServiceType, *info.Type) != 0 {
						return fmt.Errorf("service type mismatch: %s != %s", *i.ServiceType, *info.Type)
					}
				}
				return nil
			}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	g, gctx := errgroup.WithContext(ctx)
	for _, id := range instance.IDs {
		id := id
		g.Go(func() error {
			return checkOnline(gctx, id)
		})
	}
	err = g.Wait()
	if err != nil {
		return fmt.Errorf("error while checking for instances to come online: %w", err)
	}
	return nil
}

func InstanceStop(ctx context.Context, message stream.Message) error {
	instance := st.InstanceStopCommand{}
	container := message.MustGet(dependency.ContainerDep).(container.Container)

	err := message.ParsePayload(&instance)
	if err != nil {
		return fmt.Errorf("error unmarshalling InstanceStop payload: %w", err)
	}

	err = container.Stop(ctx, instance.ID, true)
	if err != nil {
		return err
	}

	return nil
}
