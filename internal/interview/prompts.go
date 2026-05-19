package interview

// Prompt templates and constants for LLM conversation context building.

// maxContextChars is the approximate character limit for conversation context.
const maxContextChars = 60000

// maxRecentMessages is how many recent messages to keep verbatim when summarizing.
const maxRecentMessages = 20

// docDelimOpen/Close are the markers for embedded requirement documents in LLM responses.
const docDelimOpen = "=== REQUIREMENT DOCUMENT ==="
const docDelimClose = "=== END REQUIREMENT DOCUMENT ==="

// ReevaluationPrompt is the template sent to the LLM when the user triggers /reevaluate.
const ReevaluationPrompt = `
You are asked to re-evaluate the requirement document for this topic from scratch.

Topic: %s
High-level requirement: %s

Current requirement document:

%s

Please review the document critically and:
1. Identify any gaps, inconsistencies, or missing sections
2. Suggest improvements to the structure and content
3. Provide an updated version of the document wrapped in the following delimiters if changes are needed:

=== REQUIREMENT DOCUMENT ===
<updated document content>
=== END REQUIREMENT DOCUMENT ===

Remember to maintain the document in markdown format and wrap any updated document in the delimiters above.`

// TopicHeader is the header line showing topic name and description.
const TopicHeader = "Topic: %s\nHigh-level requirement: %s\n"

// UploadedDocsHeader introduces uploaded reference documents in the context.
const UploadedDocsHeader = "\nUploaded reference documents:\n"

// AttachmentHeader is the per-attachment header line.
const AttachmentHeader = "\n[Document: %s (%s, %d bytes)]\n"

// CurrentDocHeader introduces the current requirement document in the context.
const CurrentDocHeader = "\nCurrent requirement document:\n"

// NoDocYet instructs the LLM to create an initial requirements document when none exists.
const NoDocYet = `
No requirement document exists yet. You should create an initial
requirements document based on what you learn from the stakeholder.
Wrap the complete document in the following delimiters:

=== REQUIREMENT DOCUMENT ===
<complete markdown document content here>
=== END REQUIREMENT DOCUMENT ===
`

// ConversationContextHeader introduces the conversation history section.
const ConversationContextHeader = "\nConversation context:\n"

// ConversationSummaryHeader introduces the summarized earlier exchanges.
const ConversationSummaryHeader = "\nConversation summary (earlier exchanges):\n"

// RecentConversationHeader introduces the recent (non-summarized) conversation.
const RecentConversationHeader = "\n\nRecent conversation:\n"

// MessageFormat is the per-message format in conversation context.
const MessageFormat = "%s: %s\n"

// FirstInteractionPrompt instructs the LLM to critically evaluate existing information
// on the first interaction before responding to the user's message.
const FirstInteractionPrompt = `
User's latest message: %s

This is the first interaction for this topic. Before responding to the user, critically evaluate the current requirement document and all available information above. Identify gaps, inconsistencies, or missing sections. Provide an updated version of the document wrapped in the following delimiters if improvements are needed:
=== REQUIREMENT DOCUMENT ===
<updated document content>
=== END REQUIREMENT DOCUMENT ===

Then address the user's message as a business analyst conducting requirements gathering.`

// UserLatestMessagePrompt appends the user's latest message with analyst instructions.
const UserLatestMessagePrompt = "\nUser's latest message: %s\n\nPlease respond as a business analyst conducting requirements gathering."

// SummaryHeader introduces a summarized message block.
const SummaryHeader = "Summary of %d earlier exchanges:\n\n"

// StakeholderKeyPointsHeader introduces key points from the stakeholder in summaries.
const StakeholderKeyPointsHeader = "Key points from stakeholder:\n"

// AnalystKeyInsightsHeader introduces key insights from the analyst in summaries.
const AnalystKeyInsightsHeader = "Key insights and questions from analyst:\n"

// SummaryPointFormat is the per-point format in summaries.
const SummaryPointFormat = "  %d. %s\n"

// NoPriorConversation is returned when there are no messages to summarize.
const NoPriorConversation = "(no prior conversation)"

// TruncatedSuffix is appended when content exceeds the context limit.
const TruncatedSuffix = "...(truncated)"

// ContextTruncatedSuffix is appended when the requirement document is truncated for context.
const ContextTruncatedSuffix = "...(truncated for context)"
