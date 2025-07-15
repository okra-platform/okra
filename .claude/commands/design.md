# Design Command

This command helps create design documentation and suggests next features to implement.

## Usage

- `/design` - Interactive prompt to specify what feature to design
- `/design suggest` - Review current system and suggest next features to implement
- `/design [feature-name]` - Create design docs for a specific feature

## Instructions

First, check what the user provided after `/design`:

1. If the user typed just `/design` with no additional text:
   - Ask them: "What feature would you like to create design docs for?"
   - If a user says something like "suggest one" or "make a suggestion" or "what do you think" follow the instructions in step 2 as if they had typed `/design suggest`
   - If a user provides something that looks like a feature name or description, create a feature-name and follow the instructions in step 3 as if they had typed `/design feature-name`

2. If the user typed `/design suggest`:
   - Review the system design (docs/*), designs (docs/design/*) and current codebase
   - Analyze what's already implemented and what design docs exist
   - Generate 3-5 specific, actionable suggestions for what to build next
   - For each suggestion, explain why it's a logical next step, the technical approach, complexity estimate, and any dependencies
   - Once the user chooses a feature, proceed to step 3 as if they had typed `/design feature-name`

3. If the user typed `/design [feature-name]` (where [feature-name] is any other text):
   - **FIRST, before doing anything else:**
     - Read all the docs in the docs/* dir to get a high-level view of the system (recursively)
     - Read any necessary code to understand the current implementation relevant to the requested feature
     - Only after understanding the system context, proceed with the following steps
   
   - Parse the feature description to extract:
     - A concise feature-name (kebab-case, suitable for file/branch names)
     - The core functionality being requested
   - Present your understanding to the user:
     ```
     I understand you want to design: [natural language description]
     
     Feature name: [kebab-case-name]
     
     My understanding: [1-2 paragraph explanation of what this feature would do,
     its purpose, key components, and how it fits into OKRA's architecture]
     
     Is this correct? Should I proceed with creating the design document?
     ```
   - Wait for user confirmation or clarification
   - Once confirmed:
     - Follow the guidelines in docs/103_design-first-approach.md to create design docs for that feature
     - After creating the design doc, update .claude/feature-status.json to add the new feature with:
       - design_date: today's date
       - design_doc: path to the design document
       - status: "design-complete"
     - Create a git branch called design-doc/<feature-name> for the design documentation

Do NOT commit changes - I will handle commits after review.

