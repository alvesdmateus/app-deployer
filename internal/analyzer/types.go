package analyzer

// Language represents a programming language
type Language string

const (
	LanguageGo         Language = "go"
	LanguageNodeJS     Language = "nodejs"
	LanguagePython     Language = "python"
	LanguageJava       Language = "java"
	LanguageRust       Language = "rust"
	LanguageRuby       Language = "ruby"
	LanguagePHP        Language = "php"
	LanguageDotNet     Language = "dotnet"
	LanguageUnknown    Language = "unknown"
)

// Framework represents a web framework
type Framework string

const (
	// Go frameworks
	FrameworkGin       Framework = "gin"
	FrameworkEcho      Framework = "echo"
	FrameworkChi       Framework = "chi"
	FrameworkFiber     Framework = "fiber"

	// Node.js frameworks
	FrameworkExpress   Framework = "express"
	FrameworkNestJS    Framework = "nestjs"
	FrameworkNextJS    Framework = "nextjs"
	FrameworkKoa       Framework = "koa"
	FrameworkFastify   Framework = "fastify"

	// Python frameworks
	FrameworkFlask     Framework = "flask"
	FrameworkDjango    Framework = "django"
	FrameworkFastAPI   Framework = "fastapi"

	// Java frameworks
	FrameworkSpringBoot Framework = "springboot"
	FrameworkQuarkus   Framework = "quarkus"

	// Other
	FrameworkUnknown   Framework = "unknown"
)

// BuildTool represents a build tool
type BuildTool string

const (
	BuildToolNPM       BuildTool = "npm"
	BuildToolYarn      BuildTool = "yarn"
	BuildToolPNPM      BuildTool = "pnpm"
	BuildToolGo        BuildTool = "go"
	BuildToolPip       BuildTool = "pip"
	BuildToolPoetry    BuildTool = "poetry"
	BuildToolMaven     BuildTool = "maven"
	BuildToolGradle    BuildTool = "gradle"
	BuildToolCargo     BuildTool = "cargo"
	BuildToolUnknown   BuildTool = "unknown"
)

// AnalysisResult represents the result of source code analysis
type AnalysisResult struct {
	Language         Language           `json:"language"`
	Framework        Framework          `json:"framework"`
	BuildTool        BuildTool          `json:"build_tool"`
	Runtime          string             `json:"runtime,omitempty"`
	Dependencies     map[string]string  `json:"dependencies,omitempty"`
	DevDependencies  map[string]string  `json:"dev_dependencies,omitempty"`
	StartCommand     string             `json:"start_command,omitempty"`
	BuildCommand     string             `json:"build_command,omitempty"`
	Port             int                `json:"port,omitempty"`
	HasDockerfile    bool               `json:"has_dockerfile"`
	Files            []string           `json:"files"`
	Confidence       float64            `json:"confidence"`
}

// FileInfo represents information about a source file
type FileInfo struct {
	Path        string
	Name        string
	Extension   string
	IsDirectory bool
	Size        int64
}
