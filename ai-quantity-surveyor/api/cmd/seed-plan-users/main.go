package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/example/ai-quantity-surveyor/api/internal/analysis"
	"github.com/example/ai-quantity-surveyor/api/internal/estimate"
	"github.com/example/ai-quantity-surveyor/api/internal/plans"
	"github.com/example/ai-quantity-surveyor/api/internal/planseed"
	"github.com/example/ai-quantity-surveyor/api/internal/project"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	config, err := planseed.FromEnvironment(os.Getenv)
	if err != nil {
		return err
	}
	if !config.Enabled {
		return fmt.Errorf("explicitly set SEED_PLAN_USERS=true to provision the three preview accounts")
	}
	if os.Getenv("DATABASE_URL") == "" {
		return fmt.Errorf("DATABASE_URL is required for deployed preview accounts")
	}
	projects, err := project.NewPostgresService(os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("could not connect to the application database; check DATABASE_URL privately")
	}
	defer projects.Close()
	repository, err := analysis.NewPostgresRepository(projects.DB())
	if err != nil {
		return fmt.Errorf("could not initialize analysis storage")
	}
	analyses := analysis.NewServiceWithEngines(analysis.DemoAnalyzer{}, map[string]analysis.Analyzer{"demo": analysis.DemoAnalyzer{}}, repository)
	if err := analyses.EnableRuntime(analysis.RuntimeConfig{Jobs: repository, Preparer: analysis.NewPagePreparer(), DefaultEngine: "demo"}); err != nil {
		return err
	}
	defer analyses.Close()
	estimateRepository, err := estimate.NewPostgresRepository(projects.DB())
	if err != nil {
		return fmt.Errorf("could not initialize estimate storage")
	}
	estimates := estimate.NewServiceWithRepository(analyses, estimateRepository)
	planStore, err := plans.NewStore(projects.DB())
	if err != nil {
		return fmt.Errorf("could not initialize plan storage")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	users, err := planseed.Run(ctx, config, projects, analyses, estimates, planStore)
	if err != nil {
		return err
	}
	for _, user := range users {
		fmt.Printf("Preview account ready: plan=%s email=%s created=%t (password never printed)\n", user.PlanID, user.Email, user.Created)
	}
	return nil
}
