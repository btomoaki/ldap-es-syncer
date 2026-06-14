# Project Rules and Guidelines for Gemini

## 1. Documentation & History Tracking (Strict Requirement)
- **Automatic History Logging:** You must create and maintain a file named `prompt_history.md` at the root of the project.
- **Execution Order:** Every time the user gives you a new step, prompt, or whenever a major design decision is mutually agreed upon, your **very first action** in your execution loop must be to append the prompt, the current date (Year 2026), and a brief summary of the decision to `prompt_history.md`.
- **Formatting:** Ensure the formatting in `prompt_history.md` is clean, sequential (e.g., Step 0, Step 1, etc.), markdown-compliant, and easy to read.
- **Write Permission Exemption:** Writing to `prompt_history.md` is exempt from the file-write confirmation rule (Rule 5.3) and must be done autonomously.

## 2. Project Structure (Clean Architecture)
Follow this directory structure strictly to ensure a technology-agnostic Domain layer:

- `internal/di/`: Dependency Injection container and wiring.
  - **Responsibility:** Initialize all providers (Infrastructure, Application, Domain) and resolve dependencies.
- `internal/domain/`: Core Business Logic
  - `model/`: Domain Entities and their associated business logic methods.
  - `repository/`: Interface definitions (Ports) for data persistence and external access.
  - `service/`: Domain Services that orchestrate complex business rules.
- `internal/application/`: Application Use Cases that coordinate the flow of data (e.g., UseCase orchestration).
- `internal/infrastructure/`: Concrete Implementations (Adapters)
  - `[provider_name]/`: Specific technology implementations (e.g., Database, External APIs, Message Brokers).
  - `config/`: Configuration and Environment variables.
    - **Responsibility:** Loading environment variables (using Go's `os` package or specialized config libraries) and providing individual configuration structs to the DI container.
- `cmd/`: Application entry points (Main functions).

## 3. Design Rules
- **Dependency Rule:** All dependencies must flow **inwards** (Infrastructure -> Application -> Domain). The Domain layer must have zero knowledge of specific technologies (e.g., no SQL, no LDAP specific logic).
- **Abstractions over Concretions:** Use interfaces defined in the Domain layer to interact with the Infrastructure layer.
- **Dependency Injection (DI):** Maintain loose coupling by injecting concrete infrastructure adapters into the application/domain via DI containers or constructors.
- **Naming Conventions:** Use generic names in the Domain layer (e.g., `UserRepository`) and specific names in the Infrastructure layer (e.g., `LdapUserRepository`).
- **Cloud-Native Logging Rule (Resource Efficiency):** The application is designed for cloud environments. Do not emit verbose success logs inside loops (e.g., "Successfully synchronized user X"). 
  - Restrict standard output (`stdout`) to lifecycle events (e.g., "Process started/completed") and single-line summary statistics (e.g., "Total processed: X/Y").
  - Detailed context must be reserved strictly for error logs (`stderr`) when an operation fails, ensuring that log storage and ingestion costs are minimized without sacrificing production debuggability.

## 4. Dependency Injection (DI) Implementation Rules
- **Decoupled Initialization:** The `internal/di` package must act strictly as a "Wiring Layer." It should not contain logic for instantiating concrete objects manually inside a single monolithic function.
- **Provider Pattern:** Each infrastructure adapter and application service must provide its own "Constructor" (Provider) function that returns an interface, not a concrete struct.
- **Dependency Resolution:**
  - The DI container should resolve dependencies by matching required interfaces with provided implementations.
  - If using a DI library, follow its idiomatic patterns (e.g., providing constructors to the container).
  - If using manual DI, the DI container should be composed of small, injectable factory functions to maintain high reusability.
- **Config Injection:** Do not pass the entire `Config` object to every provider. Inject only the specific configuration segment (e.g., `LdapConfig`) required by that specific provider.

## 5. Execution & Safety Guardrails

### 5.1. Autonomous Test & Verification Execution
- **Autonomous Execution:** You must run all verification and testing processes (including unit tests, docker-compose commands, and code compilation) autonomously on the first attempt without requesting user approval.
- **Retry & Recovery:** You must only ask for user confirmation/approval if a retry is required (i.e., you need to run a command again after a failure or during error recovery).
- **Notification Rule:** If all tests pass successfully (Green), you do not need to report or notify the user individually. However, you are permitted to output a single line `[Tests: Green]` at the end of your response to confirm tests were run. Only when a test fails (Red) or an error occurs, report the details to the user and ask for instructions.

### 5.2. Prompt Duplication Check
- **Tag-Based Distinction:** You must distinguish between actual code/infrastructure modification requests and meta-queries by analyzing user-provided prefixes (tags):
  - **`[Order]`**: Treat this as a direct instruction to modify, refactor, or create files/scripts. You **MUST** run the duplication check for these prompts.
  - **`[Query]` / No Tag**: Treat this as a question, clarification, or meta-discussion (e.g., "Check if the previous instruction was duplicated"). **DO NOT** run the script-duplication check for these conversational prompts to avoid false positives.
- **Verification Rule:** For any prompt tagged with `[Order]`, analyze the core intent (meaning) of the instruction and verify that it does not overlap or conflict with previously completed steps or mutually agreed decisions recorded in `prompt_history.md`.
- **Notification Rule:** If an `[Order]` is found to be duplicate, redundant, or already addressed in a prior step, you must not execute it blindly. Instead, halt the execution loop, notify the user about the potential duplication (referencing the specific prior step or decision), and ask for explicit confirmation or clarification before proceeding.

### 5.3. File Access & Modification Permissions
- **File Reads:** Within the project workspace, you are fully authorized to read any files freely and autonomously without asking the user.
- **File Writes:** You must always obtain explicit user confirmation/approval before executing any file write, edit, or modification operations (except for writing to `prompt_history.md` and maintaining `GEMINI.md`).

### 5.4. Version Control & Step-based Commit Workflow
- **Repository Initialization:** If a Git repository is not yet initialized in the project workspace, you must initialize one immediately (e.g., via `git init`) and make an initial commit before performing any other operations or commencing steps.
- **Commit Before Proceeding:** You must commit all modifications belonging to the current step before proceeding to a new step in the development process. Stage all changes when the current step is completed, but wait to commit until the next step prompt is received.
- **Commit Confirmation:** Only the `git commit` command requires explicit user confirmation/approval before execution. Other auxiliary Git commands (such as `git status`, `git diff`, and `git add`) may be executed autonomously on the first attempt without asking for approval.
- **Rollback and Recovery:** In case of regression, failure, or major changes in direction during a step, you must use `git reset` (or `git reset --hard` if discarding changes) to revert the workspace to the clean state at the beginning of that step before restarting your implementation.
