golang web application to capture business requirement using a interview process
the application uses a external LLM model to an agent to act as business analyst
it conduct a interview process in order to tense out the domain knowledge or edge cases of the requirement.
it should support multiple topics and each topics maintins a independent chat / interview session.  
Each topic is driven from a topic requirement document.  The Agent will regular update the requirement document 
the requirement document uses markdown format

standalone golang web application
uses the fiber web framework
use https://github.com/cloudwego/eino to talk to the external llm model
the llm model should be configurable
all configuration is stored in a json file
