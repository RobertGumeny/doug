import { complete, type Message } from "@mariozechner/pi-ai";
import type { ExtensionAPI, SessionEntry } from "@mariozechner/pi-coding-agent";
import { BorderedLoader, convertToLlm, serializeConversation } from "@mariozechner/pi-coding-agent";

const SYSTEM_PROMPT = `You create handoff prompts for a fresh coding-agent session.

Given the prior conversation and the user's new goal:
- Summarize only the context that matters for that goal
- Emphasize decisions, constraints, discoveries, and unfinished work relevant to the goal
- Mention important files, commands, errors, or artifacts only when they help with the goal
- Omit irrelevant history
- End with a clear task for the new session

Return a self-contained prompt for the new session using exactly this structure:

## Goal
<restate the user's goal clearly>

## Relevant Context
<brief, goal-driven summary>

## Important Artifacts
- <files, commands, outputs, or notes if relevant>

## Next Step
<what the new session should do now>

Do not add any preamble or explanation outside that prompt.`;

const normalizeGoal = (raw: string): string => {
	const trimmed = raw.trim();
	if ((trimmed.startsWith('"') && trimmed.endsWith('"')) || (trimmed.startsWith("'") && trimmed.endsWith("'"))) {
		return trimmed.slice(1, -1).trim();
	}
	return trimmed;
};

export default function (pi: ExtensionAPI) {
	pi.registerCommand("handoff", {
		description: "Summarize this session for a goal and continue in a new session",
		handler: async (args, ctx) => {
			if (!ctx.hasUI) {
				ctx.ui.notify("/handoff requires interactive mode", "error");
				return;
			}

			if (!ctx.model) {
				ctx.ui.notify("No model selected", "error");
				return;
			}

			const goal = normalizeGoal(args);
			if (!goal) {
				ctx.ui.notify('Usage: /handoff "goal for the new session"', "warning");
				return;
			}

			const branch = ctx.sessionManager.getBranch();
			const messages = branch
				.filter((entry): entry is SessionEntry & { type: "message" } => entry.type === "message")
				.map((entry) => entry.message);

			if (messages.length === 0) {
				ctx.ui.notify("No conversation to hand off", "warning");
				return;
			}

			const conversationText = serializeConversation(convertToLlm(messages));
			const currentSessionFile = ctx.sessionManager.getSessionFile();

			const generatedPrompt = await ctx.ui.custom<string | null>((tui, theme, _kb, done) => {
				const loader = new BorderedLoader(tui, theme, "Generating handoff...");
				loader.onAbort = () => done(null);

				const run = async () => {
					const auth = await ctx.modelRegistry.getApiKeyAndHeaders(ctx.model!);
					if (!auth.ok || !auth.apiKey) {
						throw new Error(auth.ok ? `No API key for ${ctx.model!.provider}` : auth.error);
					}

					const userMessage: Message = {
						role: "user",
						content: [
							{
								type: "text",
								text: `## Goal\n${goal}\n\n## Conversation History\n${conversationText}`,
							},
						],
						timestamp: Date.now(),
					};

					const response = await complete(
						ctx.model!,
						{ systemPrompt: SYSTEM_PROMPT, messages: [userMessage] },
						{ apiKey: auth.apiKey, headers: auth.headers, signal: loader.signal },
					);

					if (response.stopReason === "aborted") {
						return null;
					}

					return response.content
						.filter((c): c is { type: "text"; text: string } => c.type === "text")
						.map((c) => c.text)
						.join("\n")
						.trim();
				};

				run().then(done).catch((error) => {
					console.error("/handoff failed", error);
					done(null);
				});

				return loader;
			});

			if (!generatedPrompt) {
				ctx.ui.notify("Handoff cancelled", "info");
				return;
			}

			const editedPrompt = await ctx.ui.editor("Review handoff prompt", generatedPrompt);
			if (editedPrompt === undefined) {
				ctx.ui.notify("Handoff cancelled", "info");
				return;
			}

			const result = await ctx.newSession({
				parentSession: currentSessionFile,
				withSession: async (replacementCtx) => {
					replacementCtx.ui.notify("Starting handoff session...", "info");
					await replacementCtx.sendUserMessage(editedPrompt);
				},
			});

			if (result.cancelled) {
				ctx.ui.notify("New session cancelled", "info");
			}
		},
	});
}
