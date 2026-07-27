#!/usr/bin/env python3
"""
SIA Harness Adapter Entry Point
Executes a SIA target agent with a prompt injected by CWSO rollout.

Environment Variables:
- CWSO_HARNESS_PROMPT: The task prompt for the agent
- ANTHROPIC_API_KEY: Claude API key (if using Claude backend)
- ANTHROPIC_BASE_URL: Claude API base URL override (for proxy routing via cwso-rollout)
- LLM_BASE_URL: Generic LLM base URL override (for openhands backend via proxy)
- OPENAI_API_KEY: OpenAI API key (if using openhands with OpenAI)
- OPENAI_BASE_URL: OpenAI API base URL override (for openhands)
- GEMINI_API_KEY: Gemini API key (if using openhands with Gemini)
- GOOGLE_API_KEY: Alias for GEMINI_API_KEY
- SIA_BACKEND: "claude" (default) or "openhands"
- SIA_MODEL: Model identifier for the target agent (e.g., "haiku" or "gemini/gemini-3.1-pro")
- SIA_MAX_TURNS: Max turns for agent execution (default: 10)

Output:
- /workspace/output.json: JSON file containing execution trajectory and results
"""

import asyncio
import json
import logging
import os
import sys
from pathlib import Path
from typing import Any

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S"
)
logger = logging.getLogger(__name__)


async def main():
    """Main entry point for SIA harness execution."""
    
    # Validate CWSO_HARNESS_PROMPT is present
    prompt = os.getenv("CWSO_HARNESS_PROMPT", "").strip()
    if not prompt:
        logger.error("CWSO_HARNESS_PROMPT environment variable not set or empty")
        sys.exit(1)
    
    # Get configuration from environment
    backend = os.getenv("SIA_BACKEND", "claude").lower()
    model = os.getenv("SIA_MODEL", "haiku" if backend == "claude" else "gemini/gemini-3.1-pro-preview")
    max_turns_str = os.getenv("SIA_MAX_TURNS", "10")
    
    try:
        max_turns = int(max_turns_str)
    except ValueError:
        logger.warning(f"Invalid SIA_MAX_TURNS={max_turns_str}, using default 10")
        max_turns = 10
    
    # Workspace directory (mounted by CWSO launcher at /workspace)
    workspace_dir = "/workspace"
    output_file = os.path.join(workspace_dir, "output.json")
    
    if not os.path.isdir(workspace_dir):
        logger.error(f"Workspace directory not found: {workspace_dir}")
        sys.exit(1)
    
    logger.info("=" * 80)
    logger.info("SIA Harness Adapter Starting")
    logger.info("=" * 80)
    logger.info(f"Backend: {backend}")
    logger.info(f"Model: {model}")
    logger.info(f"Max turns: {max_turns}")
    logger.info(f"Workspace: {workspace_dir}")
    logger.info(f"Output file: {output_file}")
    logger.info("=" * 80)
    
    # Import SIA utilities
    try:
        from sia.util import run_agent
    except ImportError as e:
        logger.error(f"Failed to import SIA: {e}")
        sys.exit(1)
    
    # Execution context
    execution_result = {
        "backend": backend,
        "model": model,
        "prompt": prompt,
        "workspace": workspace_dir,
        "status": "pending",
        "error": None,
        "trajectory": None,
    }
    
    try:
        logger.info(f"Starting agent with prompt:\n{prompt}\n")
        
        # Run the agent
        await run_agent(
            model_name=model,
            max_turns=max_turns,
            prompt=prompt,
            agent_working_directory=workspace_dir,
            backend=backend
        )
        
        execution_result["status"] = "success"
        logger.info("Agent execution completed successfully")
        
    except Exception as e:
        execution_result["status"] = "error"
        execution_result["error"] = str(e)
        logger.error(f"Agent execution failed: {e}", exc_info=True)
        sys.exit(1)
    
    # Write output
    try:
        with open(output_file, "w") as f:
            json.dump(execution_result, f, indent=2)
        logger.info(f"Output written to {output_file}")
    except Exception as e:
        logger.error(f"Failed to write output file: {e}")
        sys.exit(1)
    
    logger.info("=" * 80)
    logger.info("SIA Harness Adapter Complete")
    logger.info("=" * 80)


if __name__ == "__main__":
    asyncio.run(main())
