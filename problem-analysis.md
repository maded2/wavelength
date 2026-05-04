# Problem Analysis: Wavelength — AI-Driven Business Requirement Gathering

## 1. Problem Statement

### 1.1 The Core Problem

Business stakeholders and product owners frequently have high-level, abstract ideas for software systems but struggle to articulate the detailed, structured requirements that development teams need to design and build those systems. Traditional requirement gathering relies on experienced business analysts (BAs) conducting manual interview sessions — a process that is:

- **Time-consuming**: Multiple sessions over days or weeks to tease out details
- **Inconsistent**: Quality depends on the BA's experience and domain familiarity  
- **Incomplete**: Edge cases, constraints, and non-functional requirements are often missed
- **Unscalable**: A single BA can only handle a limited number of concurrent engagements
- **Costly**: Requires skilled personnel throughout the elicitation process

The goal is to create a web application where an AI agent, powered by a configurable Large Language Model (LLM), assumes the role of a business analyst and conducts a structured, interview-style conversation with stakeholders to progressively elicit, refine, and document detailed requirements — starting from nothing more than a high-level idea.

### 1.2 Context: A Meta-Project

This application is itself a software project whose requirements are being defined. The `requirement.md` file in the repository represents both the specification for the application AND the first example of the type of document the application will produce and maintain. The `prompt.txt` file captures the intended behavior and persona of the AI agent that will power the interview process.

---

## 2. Stakeholder Analysis

### 2.1 Primary Stakeholders

| Stakeholder | Role | Core Needs |
|---|---|---|
| **Product Owner / Idea Originator** | The person with a high-level business need who initiates the requirement-gathering process | To transform a vague idea into a well-structured, detailed requirement document through guided conversation, without needing BA expertise themselves |
| **AI Agent (LLM-powered BA)** | The automated interviewer and documenter | To ask relevant, probing questions; identify gaps, contradictions, and edge cases; and synthesize answers into a coherent requirement document |
| **Topic Stakeholder (invited participant)** | Additional domain experts or stakeholders who may participate in an interview session for a specific topic | To contribute domain-specific knowledge and validate requirements in a focused session |

### 2.2 Secondary Stakeholders

| Stakeholder | Role | Core Needs |
|---|---|---|
| **Development Team** | Consumers of the finalized requirement documents | To receive clear, complete, unambiguous requirement documents in a standard format (markdown) that can inform design and implementation |
| **Project Manager / Scrum Master** | Overseer of the requirement gathering process | To track the state of requirement elicitation across multiple topics; to know which topics are well-defined and which need more attention |
| **System Administrator / Operator** | Person deploying and configuring the application | To configure which LLM model is used, manage the configuration file, and operate the standalone application |

---

## 3. User Needs and Pain Points

### 3.1 Pain Points Being Addressed

1. **"Blank page" paralysis**: Stakeholders know what they want in broad terms but freeze when asked to write a detailed specification. The interview process guides them incrementally.

2. **Forgotten edge cases**: Human BAs and stakeholders often overlook what happens at boundaries, under failure conditions, or with unusual inputs. The AI agent systematically probes for these.

3. **Domain knowledge tacit assumptions**: Domain experts often assume knowledge that developers don't have. The AI agent forces explicit articulation.

4. **Requirement drift and inconsistency**: As requirements evolve across conversations, early decisions may conflict with later ones. The AI agent circles back to reconcile contradictions.

5. **Multiple parallel initiatives**: Organizations often have several concurrent requirement-gathering efforts for different features/products. Each needs isolated, independent tracking.

6. **Documentation lag**: Traditional processes have a gap between the conversation and the written spec. Here the document is updated continuously during the interview.

### 3.2 User Journey (High-Level)

1. **Initiation**: A user opens the application, creates a new "topic" (requirement-gathering initiative), and provides a brief, high-level description of what they want.
2. **Interview**: The AI agent begins asking questions — about users, goals, workflows, constraints, data, integrations, non-functional needs, edge cases.
3. **Iterative refinement**: The user answers questions; the AI agent asks follow-ups, identifies gaps, and updates the requirement document in real-time.
4. **Document evolution**: The markdown requirement document grows from a sparse outline into a detailed specification as the conversation progresses.
5. **Circling back**: Periodically, the AI agent re-evaluates earlier sections for consistency with newly elicited information.
6. **Session management**: The user can pause and resume; multiple topics can be in progress simultaneously with independent histories.
7. **Completion**: When the AI agent determines the requirement is sufficiently detailed, the process can conclude (or the user can end it).

---

## 4. Functional Requirements

### 4.1 Topic Management

- **FR-01**: The system shall allow users to create multiple independent "topics," each representing a distinct requirement-gathering initiative.
- **FR-02**: Each topic shall maintain its own isolated chat/interview session with independent conversation history.
- **FR-03**: The system shall allow users to list, view, and switch between active topics.
- **FR-04**: Each topic shall be associated with exactly one requirement document (markdown format) that evolves during the interview process.
- **FR-05**: Topics shall be persistent — their state (conversation history and requirement document) must survive application restarts.

### 4.2 Interview Process

- **FR-06**: The AI agent (acting as a business analyst) shall initiate and conduct an interview-style conversation with the user for each topic.
- **FR-07**: The AI agent shall start from the high-level requirement provided by the user and progressively drill into details through targeted questions.
- **FR-08**: The AI agent shall probe for: user personas/actors, functional workflows, business rules, constraints, edge cases, non-functional requirements, dependencies, and assumptions.
- **FR-09**: The AI agent shall identify gaps, ambiguities, and contradictions in the evolving requirement and ask clarifying questions.
- **FR-10**: The AI agent shall periodically "circle back" to re-evaluate earlier conclusions in light of later discoveries.
- **FR-11**: The user shall interact with the AI agent through a text-based chat interface within the web application.

### 4.3 Requirement Document Management

- **FR-12**: Each topic shall have a requirement document stored in markdown format.
- **FR-13**: The AI agent shall regularly update the requirement document based on the ongoing interview conversation.
- **FR-14**: The requirement document shall reflect the current state of all elicited requirements, including any decisions or choices made during the interview.
- **FR-15**: Users shall be able to view the current state of the requirement document at any time.
- **FR-16**: The requirement document shall be the output/deliverable artifact of the interview process.

### 4.4 LLM Configuration

- **FR-17**: The system shall communicate with an external LLM through a configurable interface.
- **FR-18**: The LLM model (type, provider, endpoint, credentials) shall be configurable without code changes.
- **FR-19**: All application configuration shall be stored in a single JSON configuration file.

### 4.5 Web Application

- **FR-20**: The system shall be a standalone web application accessible via a web browser.
- **FR-21**: The application shall support multiple concurrent users/topics.

---

## 5. Non-Functional Requirements

### 5.1 Usability

- **NFR-01**: The interview interface shall feel conversational and natural, not like filling out a form.
- **NFR-02**: The requirement document shall be presented in readable, well-structured markdown.
- **NFR-03**: Users with no technical knowledge of LLMs or BAs shall be able to use the application effectively.

### 5.2 Performance

- **NFR-04**: AI agent responses during the interview shall feel reasonably responsive (exact acceptable latency to be determined).
- **NFR-05**: The system shall handle multiple concurrent topics without significant degradation.

### 5.3 Reliability & Robustness

- **NFR-06**: If the external LLM service is unavailable, the system shall handle the failure gracefully (e.g., inform the user, allow retry, preserve all existing state).
- **NFR-07**: Conversation history and requirement documents shall not be lost due to application or LLM service interruptions.
- **NFR-08**: The system shall continue functioning for other topics if one topic's LLM interaction fails.

### 5.4 Configurability & Operability

- **NFR-09**: Configuration via a single JSON file — no database setup, no environment variables required for core operation.
- **NFR-10**: The application shall be runnable as a single standalone binary or process.

### 5.5 Maintainability & Extensibility

- **NFR-11**: The AI agent's behavior (its interview style, questioning strategy) should be modifiable through prompt configuration (as exemplified by `prompt.txt`), not hardcoded logic.
- **NFR-12**: The LLM backend shall be swappable via configuration to support different models/providers.

---

## 6. Business Rules

- **BR-01**: Each topic is self-contained — conversations and documents from one topic must not influence or leak into another topic.
- **BR-02**: The AI agent's role is that of a business analyst working for the IT department, interviewing stakeholders — it is NOT a developer, architect, or solution designer. Its focus is on requirements, not implementation.
- **BR-03**: The interview starts from the user's initial high-level requirement text — the system does not presuppose any domain or structure.
- **BR-04**: The requirement document is a living artifact that evolves throughout the interview — it is not a final static document produced only at the end.
- **BR-05**: The AI agent may ask about implementation constraints only insofar as they inform requirements; it should not design the solution.

---

## 7. Constraints

### 7.1 Technical Constraints

- **C-01**: The application must be written in Go (Golang).
- **C-02**: The web framework must be Fiber (`github.com/gofiber/fiber`).
- **C-03**: Communication with the external LLM must use `github.com/cloudwego/eino`.
- **C-04**: Configuration must be in a single JSON file.
- **C-05**: The application is standalone — no external databases, no microservices, no message queues (unless implicitly provided by the frameworks).

### 7.2 Domain Constraints

- **C-06**: The LLM agent's "knowledge" is limited to what the model has been trained on and the context provided in the interview — it has no access to external knowledge bases or organizational data.
- **C-07**: The application does not validate the correctness of requirements — it only facilitates elicitation and documentation. The user is ultimately responsible for the accuracy of the requirements.

---

## 8. Scope Boundaries

### 8.1 In Scope

- Web-based chat interface for conducting requirement interviews
- AI-driven questioning and probing based on initial high-level requirements
- Multi-topic support with isolated sessions
- Live-evolving markdown requirement documents per topic
- Configurable LLM backend via JSON configuration
- Persistence of topics, conversations, and documents

### 8.2 Out of Scope (Explicit)

- Implementation of the requirements gathered (the app gathers requirements; it doesn't build the system)
- Integration with project management tools (JIRA, Trello, etc.)
- User authentication or role-based access control (unless later determined necessary)
- Multi-user collaborative editing of the same topic requirement document
- Export to formats other than markdown
- Version history / diff of requirement document changes (beyond what the AI agent manages in-context)
- Source control integration for requirement documents
- Training or fine-tuning the LLM model

---

## 9. Open Questions & Assumptions Requiring Validation

### 9.1 Topic Lifecycle

- **Q-01**: How is a topic "completed"? Does the AI agent determine when enough information has been gathered, or does the user manually mark it as complete?
- **Q-02**: Can a "completed" topic be reopened for further refinement?
- **Q-03**: Can a topic be deleted? If so, what happens to its conversation history and requirement document?

### 9.2 Persistence & State

- **Q-04**: Where and how are conversation histories and requirement documents persisted? (Files on disk? Embedded store? The requirement states "standalone" — does this mean file-based persistence?)
- **Q-05**: How long should topics persist? Indefinitely until deleted? With a configurable TTL?

### 9.3 LLM Interaction

- **Q-06**: What specific LLM providers/models are expected to be supported? (The configurable nature implies multiple, but what is the minimum viable set?)
- **Q-07**: How is the AI agent's "persona" prompt provided? Is it part of the configuration, a separate file, or embedded?
- **Q-08**: How frequently should the AI agent update the requirement document? After every exchange? At defined intervals? When the agent determines a meaningful change has occurred?
- **Q-09**: What is the expected behavior if the LLM response is nonsensical, off-topic, or fails to follow the agent persona? How should the system detect and handle this?

### 9.4 User Interaction

- **Q-10**: Does the user type free-form responses, or are there structured interaction patterns (e.g., confirmations, multiple-choice clarifications)?
- **Q-11**: Can the user manually edit the requirement document directly, or is it exclusively managed by the AI agent?
- **Q-12**: Can the user provide a pre-existing requirement document as the starting point rather than a blank slate?
- **Q-13**: Is there a mechanism for the user to provide initial context beyond the high-level requirement (e.g., "this is for a banking system," "this must comply with GDPR")?

### 9.5 Configuration

- **Q-14**: What specific LLM configuration parameters are needed? (Model name, API endpoint, API key, temperature, max tokens, system prompt, etc.)
- **Q-15**: Does the configuration file need to support multiple LLM configurations (e.g., different models for different topics)?

### 9.6 Web Application

- **Q-16**: Is the web interface single-page or multi-page? What are the key views/pages?
- **Q-17**: Is there a need for real-time updates (e.g., the requirement document updating live as the AI agent thinks), or is a request-response model sufficient?
- **Q-18**: Should the application serve static assets (HTML, CSS, JS) itself, or is it a pure API backend with a separate frontend?

---

## 10. Risks and Unknowns

### 10.1 LLM-Related Risks

- **Risk-01 (Quality)**: The LLM may not ask sufficiently deep or domain-appropriate questions, leading to shallow requirement documents that miss critical details.
- **Risk-02 (Hallucination)**: The LLM may fabricate requirements, invent stakeholders, or introduce incorrect assumptions into the requirement document.
- **Risk-03 (Consistency)**: The LLM may contradict itself across different parts of the requirement document, especially in long conversations.
- **Risk-04 (Context Window)**: LLMs have finite context windows. Long interview sessions with extensive conversation history plus a growing requirement document may exceed the model's context limit, leading to degraded performance or lost context.
- **Risk-05 (Latency)**: LLM response times can vary significantly. Slow responses may break the conversational flow and frustrate users.
- **Risk-06 (Cost)**: If using a paid LLM API, each interview session incurs token costs. Long, multi-turn sessions could become expensive.

### 10.2 Domain & Usage Risks

- **Risk-07 (Scope Creep)**: The AI agent might expand the scope of inquiry beyond what the user intended, leading to requirement bloat.
- **Risk-08 (User Abandonment)**: Users may start topics and never complete them, leaving incomplete documents that could mislead if consumed later.
- **Risk-09 (Over-reliance)**: Stakeholders may treat the AI-produced requirement document as authoritative without critical review, leading to flawed downstream development.

### 10.3 Technical Risks

- **Risk-10 (eino API Stability)**: The `cloudwego/eino` library may have breaking API changes or insufficient documentation, given it may be a newer/evolving library.
- **Risk-11 (Persistence Reliability)**: File-based persistence (if chosen for "standalone" operation) may be vulnerable to corruption, especially under concurrent access.
- **Risk-12 (Concurrency Issues)**: Multiple concurrent topics and LLM calls may introduce race conditions if state management is not carefully designed.

---

## 11. Success Criteria

### 11.1 Primary Success Metrics

- **SC-01**: A user with only a high-level, one-paragraph idea can, through the interview process, produce a multi-section, detailed requirement document covering functional requirements, user roles, edge cases, and constraints.
- **SC-02**: The resulting requirement document is judged by an experienced human BA or developer to be sufficiently detailed and structured to begin system design.
- **SC-03**: Users report that the interview process surfaced considerations (edge cases, constraints, user roles) they had not initially thought of.
- **SC-04**: Multiple independent topics can be managed simultaneously without cross-contamination of context or data.
- **SC-05**: The LLM backend can be changed (e.g., from one provider to another) by editing the JSON configuration file and restarting the application — no code changes required.

### 11.2 Secondary Success Metrics

- **SC-06**: The conversational flow feels natural and guided, not robotic or repetitive.
- **SC-07**: The markdown requirement document is well-formatted, readable, and logically organized.
- **SC-08**: The application can be started with minimal setup (a single binary and a configuration file).

---

## 12. Domain Model Sketch (Conceptual)

### 12.1 Key Concepts

| Concept | Description |
|---|---|
| **Topic** | A distinct requirement-gathering initiative. Has its own conversation history and requirement document. |
| **Conversation** | The ongoing interview dialogue between the user and the AI agent for a specific topic. Composed of a sequence of messages. |
| **Message** | A single turn in the conversation — either a question/probe from the AI agent or a response from the user. |
| **Requirement Document** | The markdown artifact that captures the evolving, structured requirements for a topic. |
| **AI Agent Persona** | The configured behavior and questioning strategy of the LLM (the "business analyst" role). |
| **LLM Backend** | The external model that powers the AI agent, defined by configuration (provider, model, endpoint, credentials). |

### 12.2 Key Relationships

- A **Topic** has exactly one **Conversation** (1:1)
- A **Topic** has exactly one **Requirement Document** (1:1)
- A **Conversation** consists of many **Messages** (1:N)
- The **Requirement Document** is updated by the **AI Agent** based on the **Conversation**
- The **AI Agent** is driven by a configurable **LLM Backend** and an **AI Agent Persona**

---

## 13. Summary

The Wavelength project addresses a genuine gap in the software development lifecycle: the difficult, expertise-intensive process of transforming vague business ideas into structured, actionable requirements. By leveraging an LLM-powered AI agent as an automated business analyst, the application aims to make thorough requirement elicitation accessible to anyone with an idea — not just those who can afford experienced BAs or those who know what questions to ask.

The application's key differentiators are:

1. **Conversational elicitation**: Not a template-filling exercise, but a guided interview that adapts to the user's inputs.
2. **Living documentation**: The requirement document evolves in real-time as the conversation unfolds.
3. **Multi-topic isolation**: Multiple requirement-gathering streams can coexist without interference.
4. **Configurable AI backend**: The LLM powering the interview can be swapped without code changes, allowing flexibility and cost control.

The primary challenges lie in managing LLM limitations (context windows, hallucinations, consistency), ensuring a high-quality interview experience, and making the standalone Go application simple to operate while maintaining reliable persistence and state management.
