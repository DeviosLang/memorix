# Agent 记忆机制设计深度研究报告：从 ChatGPT 逆向到 Claude Code 的分层架构

## Executive Summary

本报告基于一篇关于 ChatGPT 记忆系统逆向工程的文案，深入研究了 Agent/OpenClaw/Claude Code 的 Memory 机制设计。核心发现是：ChatGPT 确实采用了纯结构化四层设计、完全不使用向量数据库，这一逆向分析已被多个独立来源证实。但更重要的启示不在于"不用向量数据库"本身，而在于记忆系统的本质是一个分层工程问题——不同类型的信息需要不同的存储和检索策略。Claude Code 用文件系统驱动的显式记忆提供了另一种范式，而主流 Agent 框架则普遍走向了混合架构。向量数据库是工具箱里的一把锤子，不是唯一的锤子，更不是万能的锤子。

---

## 一、文案核心论点的验证与深化

文案的核心主张可以概括为三点：(1) ChatGPT 没有使用向量数据库；(2) 它采用了四层纯结构化设计；(3) 向量数据库只适合四种记忆类型中的一种。通过对多个独立逆向工程分析的交叉验证，这些主张的准确性可以得到确认。

Manthan Gupta（2025年12月）、Shlok Khemani（2025年9月）和 James Berry/LLMrefs（2025年12月）三位研究者通过独立的对话实验，分别从系统提示词中提取了 ChatGPT 的记忆架构信息。三者的结论高度一致：ChatGPT 确实没有使用 RAG 检索、没有 embedding 召回、没有向量数据库，而是采用"预计算轻量级摘要 + 显式事实存储"的方式，将所有记忆内容直接拼接进系统提示词。

不过，文案中的四层描述需要修正和补充。实际逆向发现的架构是五层（包含系统指令层），并且各层的细节比文案描述的更丰富。具体而言：

第一层是系统与开发者指令，定义行为规则和安全准则，这不属于"记忆"但构成上下文的基础。第二层是会话元数据，包含设备类型、浏览器 User Agent、屏幕尺寸、深色模式状态、订阅级别、账户年龄、时区，甚至包括过去1/7/30天的活跃天数、平均消息长度、模型使用分布等使用习惯数据。这层数据仅在会话开始时注入一次，不长期存储，主要用于实时调整回复风格。第三层是用户记忆，即结构化的长期事实档案。实验中发现约33条此类信息，包括姓名、职业、目标、偏好等。这些信息通过两种方式触发存储：用户显式说"记住这个"，或者模型检测到符合标准的事实且用户未反对。第四层是近期对话摘要，包含最近约15条对话的时间戳、标题和用户消息片段（不包含AI回复）。这层采用预计算摘要替代了RAG检索。第五层是当前会话的滑动窗口，基于Token数量管理，超出上限时丢弃旧消息但保留上层的记忆和摘要。

文案提出的"向量检索是模糊匹配，关键事实需要精确调用"这一论点得到了充分验证。Shlok Khemani 在分析中引用了 Rich Sutton 的"苦涩的教训"（Bitter Lesson），指出 OpenAI 的技术赌注是：随着上下文窗口持续扩展（GPT-5 已达256K，GPT-5.4 已达1M tokens）和处理成本持续下降，"全量发送所有记忆"将成为最优方案，根本不需要复杂的检索系统。这是一个依赖计算能力提升而非工程巧妙性的战略选择。

但这个验证也揭示了文案未提及的重要问题。ChatGPT 的记忆容量极为有限——总字数仅约1200-1500个单词，约100-200个独立条目。2025年发生了两次大规模数据丢失事故（2月5日和11月6-7日），暴露了系统的脆弱性。Hacker News 社区普遍反馈全局记忆导致的"上下文污染"问题严重。这些缺陷表明，纯结构化方案虽然简洁高效，但在可靠性和容量方面存在真实挑战。

---

## 二、Claude Code 的记忆机制：另一种哲学

如果说 ChatGPT 代表了"自动化+黑盒"的记忆哲学，Claude Code 则代表了"显式化+工具化"的对立面。

Claude Code 的记忆系统建立在两个互补机制之上：用户编写的 CLAUDE.md 文件和 Claude 自动积累的 Auto Memory。每次新会话启动时，系统按优先级顺序读取多层 Markdown 文件，将内容直接注入系统提示。这不是"个性设置"，而是行为协调机制——定义"何时行动、提问或编辑"，而非"听起来像谁"。

CLAUDE.md 支持四层层级结构。最底层是企业/组织策略（存放在系统目录下），作用于组织内所有用户和项目。其上是全局用户记忆（`~/.claude/CLAUDE.md`），作用于该用户的所有项目，建议控制在50行以内。再上是项目记忆（项目根目录的 CLAUDE.md），作用于当前仓库，可通过版本控制共享给团队，建议20-200行。最顶层是模块化规则（`.claude/rules/*.md`），通过 YAML frontmatter 的 paths 字段实现路径特定加载——例如编辑 `.tsx` 文件时自动加载 React 规则，编辑 `.test.ts` 时加载测试规则。后者优先级覆盖前者。

Auto Memory 则是 Claude 在工作过程中自动记录的学习内容。存储在 `~/.claude/projects/<project>/memory/MEMORY.md`，每次会话开始时读取前200行。内容包括项目特定的构建命令、常见错误模式和解决方案、开发偏好和架构决策记录。用户可通过 `/memory` 命令进行管理。

在 API 层面，Anthropic 还提供了 Memory Tool，允许 Claude 在对话之间通过 `/memories` 目录进行 CRUD 操作。这个工具在客户端运行，数据位置完全由用户控制。当与上下文编辑和服务器端压缩结合使用时，可以实现超出上下文限制的长时间代理工作流——对话接近清除阈值时 Claude 自动收到警告并将重要信息保存到内存文件，被清除后随时可从文件中恢复。

两种方案的核心差异可以从架构哲学层面理解。ChatGPT 是有状态的（Stateful），预加载用户档案和历史，全自动后台运行，存储 AI 生成的摘要，用户感知不到检索过程。Claude 是无状态的（Stateless），白板模式从零开始，显式调用记忆工具，存储原始对话历史和项目规则，用户可见搜索过程。ChatGPT 面向消费者市场追求便利性和零摩擦体验，Claude 面向专业开发者追求透明性和控制权。这不是技术能力的差异，而是设计哲学的选择——是"魔法般的自动化"还是"精确的工具化"。

社区实践表明，Claude 的方案在团队协作编码、长期代码库维护和规范驱动的工作流中具有显著优势，特别是 CLAUDE.md 可以纳入 Git 版本控制并在团队间共享规则。但其学习曲线对非技术用户更陡，在快速原型制作和个人日常助手场景中不如 ChatGPT 方便。

---

## 三、主流 Agent 框架的记忆设计对比

在 ChatGPT 的"纯结构化"和 Claude Code 的"文件系统驱动"之外，主流 Agent 框架提供了更多的设计选择，但它们普遍依赖向量数据库，形成了有意思的对比。

LangChain 提供了最大的灵活性。它将 Memory 作为可插拔组件，开发者可以自由选择存储后端。四种核心 Memory 类型分别解决不同问题：ConversationBufferMemory 原封不动存储所有历史但 Token 无限增长；ConversationBufferWindowMemory 只保留最近 k 轮在保持上下文和控制成本间平衡；ConversationTokenBufferMemory 按 Token 总数截断实现精确成本控制；ConversationSummaryMemory 用 LLM 动态总结长历史极大节省空间但增加额外调用成本。这四种类型实际上就是文案中提到的分层思想的工程实现。

AutoGPT 设计了内置的短期和长期记忆，使用向量存储管理上下文以防止自主递归循环中的无限循环。CrewAI 采用结构化实体与角色记忆，模仿人类团队中成员基于各自角色记忆特定信息的方式。MetaGPT 通过工作流状态和文档存储实现隐式状态管理。OpenAI Agents SDK 采用 Session 式设计，API 简洁但高级记忆支持较弱。Microsoft AutoGen 提供了多模式内存管理，参考架构完善，支持向量库和图数据库的混合使用。

Mem0 是一个专门围绕"记忆系统"构建的框架，值得特别关注。它的核心创新在于用 LLM 驱动记忆决策：新提取的记忆和相似的历史记忆同时输入 LLM，由 LLM 判断应该执行 ADD、UPDATE、DELETE 还是 NONE 操作。这解决了文案中提到的"新值覆盖旧值"问题，但代价是每次写入都需要额外的 LLM 调用。Mem0 同时支持向量存储和图存储，通过 user_id、agent_id、run_id 实现多维度隔离。

---

## 四、向量数据库 vs 结构化存储：适用边界分析

文案的核心观点——"向量数据库只是四种记忆类型中一种的解法"——从工程角度看是正确的，但需要更精确的表述。

向量数据库的真正优势在于处理模糊的、开放的、难以穷举的非结构化内容。当数据量大（数万条历史工单）、查询意图模糊（"之前聊过的关于 Python 的话题"）、表述差异大（同义词、释义）时，语义检索是最佳选择。但它的局限性同样明显：缺乏多步推理能力，在密集互联的事实数据中可能返回"相关但不精确"的结果污染上下文窗口，且无法原生处理时间序列和状态更新。

结构化存储（键值存储、关系数据库）的优势在于精确性、低延迟和可预测性。用户的预算、身份、上次选择的方案——这些是事实，不是语义相似度问题。文案中"明知钥匙在左口袋却非要翻遍所有口袋"的比喻准确描述了在这类场景下使用向量检索的冗余。

图数据库则擅长处理实体间的复杂关系和多跳推理。当需要回答"批准了预算X的管理者的所有直接下属"这类结构化问题时，图遍历的确定性路径远优于向量的近似匹配。

生产级系统越来越趋向混合架构：用键值存储作为状态层（快速存取当前状态和用户偏好），用向量存储作为回忆层（检索相关经验和历史对话），用图存储作为推理层（处理复杂关系和逻辑链）。检索时可以采用级联策略（先图库精确查询，未命中再向量库语义匹配）、并行策略（多源同时查询，合并去重排序）或加权策略（综合向量相似度、时间衰减、访问频率和重要性评分）。

---

## 五、记忆系统的分层设计原则

综合 ChatGPT、Claude Code 和主流框架的实践，可以提炼出记忆系统设计的通用原则。

第一个原则是按记忆类型分层。这与文案的四种分类一致，但需要更精细的工程实现：短期记忆（当前对话上下文）用滑动窗口管理，不需要持久化；工作记忆（最近N轮对话或摘要）用内存缓存或轻量摘要，访问频率中等；长期事实（用户档案、偏好、目标）用结构化存储，支持精确查询和覆盖更新；历史经验（过去的成功方案、失败尝试）用向量检索或图存储，支持语义搜索和关系推理。

第二个原则是记忆的生命周期管理。创建阶段需要从原始文本中提取事实、进行去重检测（向量相似度 > 0.9）和冲突检测。更新阶段需要 LLM 判断是补充、替代还是忽略新信息，并记录时间戳支持版本管理。遗忘机制同样重要——基于时间的遗忘（超过N天未使用的记忆降权）、基于重要性的遗忘（不重要的自动清理）和基于容量的遗忘（超限时清理最老最不重要的条目）。ChatGPT 的两次数据丢失事故警示我们，遗忘应该是可控的设计，而非不可控的事故。

第三个原则是上下文工程优于信息堆砌。Anthropic 的工程博客反复强调："更大的上下文窗口并不总是更好——随着token数量增加，准确性和召回率会下降（Context Rot）。"因此，精心策划注入什么内容与拥有多大的可用空间同样重要。ChatGPT 用仅1200-1500字的记忆容量服务数亿用户，正是这一原则的极端体现。

---

## 六、针对不同场景的设计建议

对于消费级AI助手（类似ChatGPT场景），推荐 ChatGPT 的四层结构化模式。核心优势是零检索延迟、固定Token消耗和工程简洁性。适合追求无缝体验的大众用户，但需要解决容量限制和数据可靠性问题。

对于开发者工具和编码助手（类似Claude Code场景），推荐文件系统驱动的显式记忆方案。CLAUDE.md + Auto Memory 的组合在团队协作、版本控制和精确控制方面具有天然优势。关键是保持记忆文件精简（200行以内）、定期审查清理过时内容、利用模块化规则避免主文件膨胀。

对于企业级Agent应用，推荐混合架构。从向量存储 + 键值缓存起步（第一阶段），随着推理需求增长加入图存储（第二阶段），最终实现语义缓存、动态遗忘等高级优化（第三阶段）。选择 Mem0 或 LangGraph 作为记忆管理框架可以减少重复造轮子的成本。

对于需要快速原型验证的场景，LangChain + ConversationBufferWindowMemory + Redis 是成本最低、上线最快的组合。验证核心假设后再考虑更复杂的记忆架构。

---

## Conclusion

回到文案最初的问题："Agent 的记忆机制应该怎么设计？"

答案确实不是"用向量数据库存历史对话，每次检索相关内容拼进去"。但也不是简单地"不用向量数据库"。正确的回答是：先搞清楚你的记忆需要存什么类型的信息。

当前对话的上下文靠滑动窗口。用户的长期事实靠结构化存储，随时覆盖，精确读取。近期对话的脉络靠轻量摘要，不需要存原文。历史经验和案例靠向量检索或图数据库。ChatGPT 证明了在大多数消费场景下前三层就够了。Claude Code 证明了文件系统也能构建强大的记忆机制。Mem0 证明了 LLM 本身可以驱动记忆的智能管理。

工程设计的本质是选对工具，而不是证明你会用复杂工具。文案中这句话，是整个记忆系统设计的最佳总结。

---

## Limitations

本研究存在以下局限性。ChatGPT 的记忆机制基于逆向工程分析而非官方文档，OpenAI 从未公开过技术实现细节，分析结果可能存在偏差。逆向分析涉及的是2025年中期之后的版本，早期版本可能采用了不同的架构。Claude Code 的记忆机制仍在快速迭代中，本报告基于2026年3月的文档和社区实践。各框架的性能对比缺乏统一的基准测试数据，大多基于社区经验和定性分析。

---

## References

1. [I Reverse Engineered ChatGPT's Memory System - Manthan Gupta](https://manthanguptaa.in/posts/chatgpt_memory/)
2. [ChatGPT Memory and the Bitter Lesson - Shlok Khemani](https://www.shloked.com/writing/chatgpt-memory-bitter-lesson)
3. [How ChatGPT Memory Works, Reverse Engineered - LLMrefs/James Berry](https://llmrefs.com/blog/reverse-engineering-chatgpt-memory)
4. [Claude Code Memory Explained: How It Really Works - Jose Parreno Garcia](https://joseparreogarcia.substack.com/p/claude-code-memory-explained)
5. [How Claude remembers your project - Claude Code Official Docs](https://code.claude.com/docs/en/memory)
6. [Context windows - Claude API Docs - Anthropic](https://platform.claude.com/docs/en/build-with-claude/context-windows)
7. [Memory tool - Claude API Docs - Anthropic](https://platform.claude.com/docs/en/agents-and-tools/tool-use/memory-tool)
8. [Claude Memory: A Different Philosophy - Shlok Khemani](https://www.shloked.com/writing/claude-memory)
9. [Vector Databases vs. Graph RAG for Agent Memory - MachineLearningMastery](https://machinelearningmastery.com/vector-databases-vs-graph-rag-for-agent-memory-when-to-use-which/)
10. [How LLM Memory Works: Architecture, Techniques, and Developer Patterns - C# Corner](https://www.c-sharpcorner.com/article/how-llm-memory-works-architecture-techniques-and-developer-patterns/)
11. [Memory For AI Agents: Vector, KV & Graph Stores - Arun Angshuda](https://arunangshudas.com/blog/ai-agent/memory-for-agents-vector-kv-graph/)
12. [AI Agent Framework Comparison: LangChain vs AutoGPT vs CrewAI - Fast.io](https://fast.io/resources/ai-agent-framework-comparison/)
13. [Comparing Agent Memory Architectures - Maxim AI](https://www.getmaxim.ai/articles/comparing-agent-memory-architectures-vector-dbs-graph-dbs-and-hybrid-approaches/)
14. [Hybrid Approaches - Agent Memory Guide](https://agentmemoryguide.ai/docs/patterns/hybrid-approaches)
15. [Memory - Multi-agent Reference Architecture - Microsoft](https://microsoft.github.io/multi-agent-reference-architecture/docs/memory/Memory.html)
16. [智能体记忆之 Mem0 - Ying](https://izualzhy.cn/agent-memory-mem0)
17. [深入解析 LangChain 的记忆（Memory）机制 - 小凌](https://shlxl.github.io/blog/engineering/2025/10/deep-dive-into-langchain-memory)
18. [OpenAI brings longer-term memory feature to free ChatGPT users - The Decoder](https://the-decoder.com/openai-brings-longer-term-memory-feature-to-free-chatgpt-users/)
19. [ChatGPT Memory Full? What's Actually Happening - Unmarkdown](https://unmarkdown.com/blog/chatgpt-memory-full)
20. [I turned off ChatGPT's memory - Hacker News Discussion](https://news.ycombinator.com/item?id=47132001)
21. [The CLAUDE.md Memory System - SFEIR Institute](https://institute.sfeir.com/en/claude-code/claude-code-memory-system-claude-md/tutorial/)
22. [Claude Code Best Practices - GitHub](https://github.com/shanraisshan/claude-code-best-practice)
23. [AI记忆系统演进 - 从ChatGPT到OpenClaw的上下文工程路线 - 阿丸笔记（微信公众号）](https://weixin.sogou.com)
24. [Manthan Gupta 破解 ChatGPT 的4层记忆机制 - 许野说（微信公众号）](https://weixin.sogou.com)
