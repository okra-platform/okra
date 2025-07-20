# Implement Command
This command implement specific features that are specified in design documentation.

## Usage
- `/implement` - Interactive prompt to choose which feature to implement
- `/implement [feature-name]` - Implement a specific feature

If the user does not provide a feature:
1. First check .claude/feature-status.json to see which features have designs but are not yet implemented
2. If the status file doesn't exist or is empty, fall back to scanning docs/design/* for design docs
3. Present a list of unimplemented features to the user with their design dates
4. Ask which feature they'd like to implement

## Instructions

When implementing a feature:

1. **FIRST, before anything else:**
   - Read all the docs in the docs/* dir to get a high-level view of the system (recursively)
   - Locate and read the design document:
     - Check .claude/feature-status.json for the exact design doc path
     - If not found there, look for docs/design/*-<feature-name>.md
     - Read the design document thoroughly
   - Read any existing code relevant to the feature implementation
   - Only after understanding the system context and design, proceed with the following steps

1a. Think deeply about the overall implementation plan, and how to build using a phased approach. We want to implement the features in an incremental phased way, with each phase being checked for accuracey, test coverage and all test passing before moving to the next. Be sure to follow the testing approach and best practices outlined in the docs for the repo.

2. Present your implementation plan to the user:
   ```
   I'll be implementing: [feature-name]
   
   Based on the design document, this feature will:
   [1-2 paragraph summary of what will be built, key components,
   and how it integrates with existing code]
   
   Main implementation tasks:
   - [Key task 1]
   - [Key task 2]
   - [Key task 3]
   
   Estimated files/directories to create/modify:
   - [path/to/file1]
   - [path/to/file2]
   
   Should I proceed with this implementation?
   ```

3. Wait for user confirmation or clarification

4. Once confirmed, implement the feature by:
   - Update .claude/feature-status.json to mark the feature as "in-progress" with today's date
   - Creating a new git branch called feature/<feature-name>
   - Implementing the feature according to the design document
   - Following all OKRA conventions in docs/100_coding-conventions.md and docs/102_testing-best-practices.md
   - Running tests after implementation
   - If tests pass, update .claude/feature-status.json to mark the feature as "implemented" with:
     - implementation_date
     - branch name
     - list of main files/directories created
     - tests_passing: true/false

Do NOT commit changes - I will handle commits after review.
