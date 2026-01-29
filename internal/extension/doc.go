// Package extension provides functionality to load custom tools and agents
// from the user's ~/.devorch directory.
//
// Directory Structure:
//
//	~/.devorch/
//	├── tools/           # Custom tool extensions
//	│   ├── mytool.json  # Tool manifest
//	│   └── mytool.py    # Tool implementation
//	├── agents/          # Custom agent extensions
//	│   ├── myagent.json # Agent manifest
//	│   └── myagent.py   # Agent implementation
//	└── config/          # Configuration files
//
// Tool Manifest Format (JSON):
//
//	{
//	  "name": "mytool",
//	  "description": "Description of what the tool does",
//	  "path": "/path/to/implementation.py",
//	  "command": "python",  // optional: interpreter command
//	  "parameters": [
//	    {
//	      "name": "param1",
//	      "type": "string",
//	      "description": "Parameter description",
//	      "required": true
//	    }
//	  ],
//	  "metadata": {
//	    "author": "Your Name",
//	    "version": "1.0.0"
//	  }
//	}
//
// Tool Implementation:
// Tools receive arguments as JSON via stdin and should output results to stdout.
// Exit code 0 indicates success, non-zero indicates failure.
package extension
