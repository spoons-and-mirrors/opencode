## IMPORTANT

- Try to keep things in one function unless composable or reusable
- DO NOT do unnecessary destructuring of variables
- DO NOT use `else` statements unless necessary
- DO NOT use `try`/`catch` if it can be avoided
- AVOID `try`/`catch` where possible
- AVOID `else` statements
- AVOID using `any` type
- AVOID `let` statements
- PREFER single word variable names where possible
- Use as many bun apis as possible like Bun.file()

## Debugging

- To test opencode in the `packages/opencode` directory you can run `bun dev`

## Assistant Agency Guideline: Batch Tool Usage

Using the `batch` tool is MANDATORY. Be proactive about it. DO NO WAIT for the user to request "the batch tool".

BATCH SHOULD BE AND _IS THE PRIMARY WAY YOU USE TOOLS IN THIS ENVIRONMENT_

When you have to perform multi-step operations such as read multiple files, spawn task subagents, edit files, bash commands... _THE USER WANTS YOU TO USE THE `batch` TOOL_ instead of separate sequential calls. This maximizes time and compute and increases your likelyhood of a promotion. Sequential tool calling is only encouraged when relying on the output of a call for the next one(s).

- WHAT CAN BE, MUST BE, AND WILL BE BATCHED
- INDIVIDUAL TOOLS TO GATHER CONTEXT IS HEAVILY DISCOURAGED (please batch those calls!)
- THE BATCH TOOL IS YOUR BEST FRIEND AND WILL INCREASE USER'S HAPPINESS
- MAKE SURE YOU USE THE BATCH TOOL PROPERLY, NEVER DEVIATE FROM THE EXPECTED PAYLOAD