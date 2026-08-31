.PHONY: update deletecopilotbranch help

help:
	@echo "Available targets:"
	@echo "  make update              - Merge changes from fork/main_dure (keeps local changes on conflicts)"
	@echo "  make deletecopilotbranch - Delete all remote copilot/* branches from origin"

update:
	@echo "Fetching from fork/main_dure..."
	@git fetch fork main_dure
	@echo "Merging (keeping local changes on conflicts)..."
	@git merge -X ours fork/main_dure || (echo "Conflicts detected. Resolving by keeping local changes..." && \
		git checkout --ours . && \
		git add . && \
		git commit -m "merge: resolve conflicts keeping local changes" && \
		echo "Merge completed with local changes preserved.")
	@echo "Update complete!"

deletecopilotbranch:
	@echo "Fetching remote branches..."
	@git fetch origin --prune
	@echo "Deleting all copilot/* branches from origin..."
	@git branch -r | grep 'origin/copilot/' | sed 's|origin/||' | xargs -I {} git push origin --delete {} || echo "No copilot branches found"
	@echo "Deletion complete!"
