# DevOrch Session Export

**Exported:** 2026-02-01 04:25:15
**Provider:** ollama
**Model:** tinyllama:latest
**Work Mode:** Ask

## Conversation

### 👤 User

Hello, how does this new DevOrch CLI work?

### 🤖 Assistant

The "devorch" command-line interface (CLI) is a powerful tool that allows developers to easily manage their projects and tasks using the DevOrch automation platform. Here's how it works:

1. Open a terminal or console window: Go to your project directory, open a terminal/console window, and type "devorch" without quotes. For example:

   ```bash
   devorch
   ```

2. Select the tasks you want to run: If DevOrch is running as the default editor, you can select tasks by typing a task name (without any "devorch" prefix), such as "devorch:start":

   ```bash
   devorch:start
   ```

   Alternatively, if DevOrch is running in your custom editor, you can also run tasks directly from the command line:

   ```bash
   devorch start
   ```

3. Run a task: Once you've selected a task, the CLI will prompt you to enter the name of the environment you want to use for that task (if it's not already set), and then launch the task automatically. If you don't specify an environment name, DevOrch will use the current one:

   ```bash
   devorch:start
   ```

   ```bash
   devorch start
   ```

4. Clean up: To finish a project, simply type "devorch clean" or "devorch:clean", and DevOrch will automatically stop all running tasks and delete any leftover files. If you're not sure what to do after finishing a project, DevOrch provides a detailed guide on how to clean up your workspace and restore your environment:

   ```bash
   devorch:clean
   ```

   ```bash
   devorch:clean --help
   ```

That's it! With "devorch" CLI, you can manage all your DevOrch automation tasks from a single console window.

### 👤 User

Can you help me with some code?

### 🤖 Assistant

Of course! Here is an example code snippet that demonstrates how to use the "devorch" CLI tool:

1. Open a new terminal/console window and navigate to your project directory.

2. Install DevOrch (if you haven't already):

   ```bash
   sudo pip install devorch
   ```

3. Create a new task:

   ```bash
   devorch create <task name>
   ```

4. Define environment variables for this task:

   ```bash
   export DEVORCH_ENVIRONMENT=<environment name>
   ```

   This will set the environment variable "DEVORCH_ENVIRONMENT" to the given value, which can be used in future commands.

5. Start a task:

   ```bash
   devorch start
   ```

   This will start a new DevOrch automaton task using the default environment, and will prompt you for the name of the environment to use for this task:

   ```bash
   Enter your environment name (default is 'dev'): 
   ```

   Enter any desired value here.

6. Run a task:

   ```bash
   devorch:start
   ```

   This will start the same DevOrch automaton task again, but using the specified environment as an existing project:

   ```bash
   Enter your environment name (default is 'dev'): 
   ```

   Enter any desired value here.

7. Clean up a project:

   ```bash
   devorch:clean
   ```

   This will clean up all DevOrch automaton tasks and their dependencies from your workspace, as well as deleting any left-over files.

8. Get detailed information on how to clean up:

   ```bash
   devorch:clean --help
   ```

   This will display a detailed guide on how to clean up DevOrch automaton tasks and their dependencies with the "devorch" CLI tool.

