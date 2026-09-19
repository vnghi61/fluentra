package main

// listeningCourseSeedData defines the 6 listening lessons across A2–B2.
//
// It exists for the same reason the speaking course does. The listening module
// has graded `listening_comprehension` since it was written, the play route
// counts plays against a learning attempt and the transcript route releases the
// script after marking — and none of it was reachable, because every listening
// item in the database belonged to `pool-exam` or `pool-placement`, which no
// learner opens on purpose. Practice offered Reading, Writing and Speaking and
// nothing to listen to.
//
// Each activity is authored twice, like reading's: `Config` is what the browser
// receives and `Body` is what the grader loads. The difference that matters is
// `script` — it is on the body only. A learner who can read the script is not
// listening, which is why the server redacts it out of the config and why the
// transcript has a route of its own that answers only after grading (ADR-0025).
//
// The audio does not exist until it is rendered. `cmd/tts -all` walks every
// published `listening_comprehension` body, synthesises its script with Piper
// and caches the clip; until that runs, these items answer the play route with
// 409 AUDIO_NOT_READY and say so in the runner. Seeding them without the render
// is the intended order, not an oversight: the render needs a voice model that
// is not in the repository.
var listeningCourseSeedData = seedCourse{
	Slug:           "listening-practice",
	Title:          "Listening Comprehension: A2–B2",
	Description:    "Announcements, conversations and short talks with comprehension questions. Each clip can be played three times, and the transcript is released after marking.",
	CEFRFrom:       "A2",
	CEFRTo:         "B2",
	EstimatedHours: 8,
	Units: []seedUnit{
		{
			Position:    1,
			Title:       "Announcements & Short Exchanges (A2)",
			Description: "Public announcements and everyday conversations, spoken slowly and clearly.",
			Lessons: []seedLesson{
				{
					Position:         1,
					Title:            "At the Station",
					SkillFocus:       skillListening,
					EstimatedMinutes: 12,
					CEFRLevel:        "a2",
					Activities: []seedActivity{
						listeningItem(
							1,
							"Platform Change",
							"Attention all passengers. The ten forty-five service to Brighton will now depart from platform six, not platform two. Passengers already waiting on platform two should make their way across the bridge. The train is running approximately five minutes late, and we apologise for the change.",
							[]listeningQuestion{
								{
									ID:     "q1",
									Type:   listeningChoice,
									Prompt: "Which platform does the train leave from now?",
									Options: []map[string]string{
										option("opt_six", "Platform six"),
										option("opt_two", "Platform two"),
										option("opt_bridge", "The platform across the bridge from six"),
										option("opt_four", "Platform four"),
									},
									Answer: "opt_six",
									EN:     "The announcement names the new platform first and the old one second. Hearing which is which is the whole question.",
									VI:     "Thông báo đọc sân ga mới trước, sân ga cũ sau. Nghe ra cái nào là cái nào chính là toàn bộ câu hỏi.",
								},
								{
									ID:     "q2",
									Type:   listeningTrueFalse,
									Prompt: "The train is running late.",
									Answer: "true",
									EN:     "Five minutes late is still late. A number does not have to be large to make the statement true.",
									VI:     "Trễ năm phút vẫn là trễ. Con số không cần lớn thì câu mới đúng.",
								},
								{
									ID:     "q3",
									Type:   listeningGapFill,
									Prompt: "The service departs at ten _____.",
									Answer: "forty-five",
									EN:     "Times said as words are where a clip is most often misheard; the answer is accepted written either way.",
									VI:     "Giờ giấc đọc thành chữ là chỗ dễ nghe nhầm nhất; đáp án chấp nhận cả hai cách viết.",
								},
								{
									ID:     "q4",
									Type:   listeningChoice,
									Prompt: "What are passengers on the old platform told to do?",
									Options: []map[string]string{
										option("opt_cross", "Cross the bridge to the new platform"),
										option("opt_wait", "Wait where they are for a later announcement"),
										option("opt_ticket", "Go back to the ticket office"),
										option("opt_exit", "Leave the station and take a bus"),
									},
									Answer: "opt_cross",
									EN:     "The instruction comes last, after the change itself. Listening to the end is the habit this question rewards.",
									VI:     "Hướng dẫn nằm ở cuối, sau phần thông báo đổi sân. Câu này thưởng cho thói quen nghe hết clip.",
								},
							},
							"Three plays, and the transcript after marking. Answer from what you heard rather than what you expected to hear.",
							"Ba lượt nghe, và bản ghi lời nói sau khi chấm. Hãy trả lời theo điều bạn nghe được, không theo điều bạn đoán sẽ nghe.",
						),
					},
				},
				{
					Position:         2,
					Title:            "Making Plans",
					SkillFocus:       skillListening,
					EstimatedMinutes: 12,
					CEFRLevel:        "a2",
					Activities: []seedActivity{
						listeningItem(
							1,
							"Saturday Afternoon",
							"Hi Mai, it's Linh. I know we said we would meet at the cafe at two, but my class finishes late on Saturdays now, so could we make it three instead? I will come straight from the university, so I might be a few minutes behind. If three does not work for you, send me a message and we will find another day. See you soon.",
							[]listeningQuestion{
								{
									ID:     "q1",
									Type:   listeningChoice,
									Prompt: "What does Linh want to change?",
									Options: []map[string]string{
										option("opt_time", "The time they are meeting"),
										option("opt_place", "The place they are meeting"),
										option("opt_day", "The day they are meeting"),
										option("opt_who", "Who is coming with them"),
									},
									Answer: "opt_time",
									EN:     "The day and the cafe both stay. Only the hour moves, which is what makes the other three wrong rather than unmentioned.",
									VI:     "Ngày và quán cà phê đều giữ nguyên. Chỉ có giờ thay đổi — đó là lý do ba phương án kia sai chứ không phải không được nhắc tới.",
								},
								{
									ID:     "q2",
									Type:   listeningGapFill,
									Prompt: "Linh suggests meeting at _____ o'clock.",
									Answer: "three",
									EN:     "Two numbers are said. The one she is proposing comes after 'instead'.",
									VI:     "Có hai con số được đọc. Con số cô ấy đề nghị nằm sau chữ \"instead\".",
								},
								{
									ID:     "q3",
									Type:   listeningTrueFalse,
									Prompt: "Linh says she may arrive slightly late.",
									Answer: "true",
									EN:     "\"A few minutes behind\" is the same thing said in other words — which is how a listening test usually says it.",
									VI:     "\"A few minutes behind\" là cách nói khác của cùng một ý — và đề nghe thường nói theo cách khác như vậy.",
								},
								{
									ID:     "q4",
									Type:   listeningChoice,
									Prompt: "What should Mai do if the new time does not suit her?",
									Options: []map[string]string{
										option("opt_msg", "Send Linh a message"),
										option("opt_call", "Call the cafe"),
										option("opt_wait", "Wait at the cafe anyway"),
										option("opt_nothing", "Do nothing; Linh will call again"),
									},
									Answer: "opt_msg",
									EN:     "The fallback is stated once, near the end, and never repeated.",
									VI:     "Phương án dự phòng chỉ được nói một lần, gần cuối, và không lặp lại.",
								},
							},
							"A voice message. Names and times carry the answers; the rest is there to make you wait for them.",
							"Một tin nhắn thoại. Tên người và giờ giấc chứa đáp án; phần còn lại có mặt để bắt bạn chờ chúng.",
						),
					},
				},
			},
		},
		{
			Position:    2,
			Title:       "Conversations & Instructions (B1)",
			Description: "Longer exchanges where the answer is implied rather than stated.",
			Lessons: []seedLesson{
				{
					Position:         1,
					Title:            "A Problem with an Order",
					SkillFocus:       skillListening,
					EstimatedMinutes: 15,
					CEFRLevel:        "b1",
					Activities: []seedActivity{
						listeningItem(
							1,
							"The Wrong Size",
							"Good afternoon, you've reached customer service. I understand you received an order last week that wasn't quite right. Let me look that up. Yes, I can see the jacket was sent in a medium, and your order says large. I'm sorry about that. What I can do is send the correct size today, and you can return the medium in the same packaging using the prepaid label inside. You won't be charged for the return, and the replacement should reach you within three working days. Is that acceptable?",
							[]listeningQuestion{
								{
									ID:     "q1",
									Type:   listeningChoice,
									Prompt: "What went wrong with the order?",
									Options: []map[string]string{
										option("opt_size", "The wrong size was sent"),
										option("opt_colour", "The wrong colour was sent"),
										option("opt_late", "The order arrived late"),
										option("opt_missing", "Part of the order was missing"),
									},
									Answer: "opt_size",
									EN:     "The two sizes are said in the same breath. Which one was sent and which was ordered is the distinction the question turns on.",
									VI:     "Hai cỡ được đọc liền nhau. Cỡ nào được gửi và cỡ nào được đặt chính là chỗ phân biệt của câu hỏi.",
								},
								{
									ID:     "q2",
									Type:   listeningTrueFalse,
									Prompt: "The customer will have to pay to return the jacket.",
									Answer: "false",
									EN:     "\"You won't be charged\" answers it directly, but only once, and a prepaid label is mentioned before that.",
									VI:     "\"You won't be charged\" trả lời trực tiếp, nhưng chỉ một lần, và nhãn trả hàng trả trước được nhắc trước đó.",
								},
								{
									ID:     "q3",
									Type:   listeningGapFill,
									Prompt: "The replacement should arrive within _____ working days.",
									Answer: "three",
									EN:     "One number in the clip is a duration and the others are not. Catching which is which is the skill being tested.",
									VI:     "Trong clip chỉ một con số là khoảng thời gian, các số khác thì không. Nhận ra số nào là số nào chính là kỹ năng được kiểm tra.",
								},
								{
									ID:     "q4",
									Type:   listeningChoice,
									Prompt: "What is the customer asked to do with the jacket they received?",
									Options: []map[string]string{
										option("opt_return", "Return it in the same packaging with the prepaid label"),
										option("opt_keep", "Keep it as well as the replacement"),
										option("opt_shop", "Take it to a shop in person"),
										option("opt_throw", "Dispose of it"),
									},
									Answer: "opt_return",
									EN:     "Two instructions are given at once — send the right size, return the wrong one. The question asks about the second.",
									VI:     "Hai hướng dẫn được đưa cùng lúc — gửi cỡ đúng, trả lại cỡ sai. Câu hỏi hỏi về cái thứ hai.",
								},
							},
							"A service call at natural speed. The answer to each question is said once; the rest of the call is politeness around it.",
							"Một cuộc gọi chăm sóc khách hàng ở tốc độ tự nhiên. Đáp án mỗi câu chỉ được nói một lần; phần còn lại là lời lẽ lịch sự bao quanh.",
						),
					},
				},
				{
					Position:         2,
					Title:            "Following Directions",
					SkillFocus:       skillListening,
					EstimatedMinutes: 15,
					CEFRLevel:        "b1",
					Activities: []seedActivity{
						listeningItem(
							1,
							"Getting to the Conference",
							"For those arriving by train, the conference centre is a fifteen-minute walk from the main station. Leave by the north exit, turn right onto Bridge Street, and continue until you reach the river. Cross the footbridge — not the road bridge, which takes you the long way round — and the centre is the glass building directly ahead. If it is raining, there is a shuttle bus from the station forecourt every twenty minutes, and it is free with your conference badge.",
							[]listeningQuestion{
								{
									ID:     "q1",
									Type:   listeningChoice,
									Prompt: "Which bridge should visitors cross?",
									Options: []map[string]string{
										option("opt_foot", "The footbridge"),
										option("opt_road", "The road bridge"),
										option("opt_either", "Either bridge"),
										option("opt_none", "Neither; they should go around"),
									},
									Answer: "opt_foot",
									EN:     "The wrong bridge is named out loud, which is what makes this worth listening twice for. A clip that mentions an option is not recommending it.",
									VI:     "Cây cầu sai được đọc to lên, nên câu này đáng nghe lại lần hai. Clip nhắc tới một lựa chọn không có nghĩa là khuyên chọn nó.",
								},
								{
									ID:     "q2",
									Type:   listeningGapFill,
									Prompt: "The walk from the station takes about _____ minutes.",
									Answer: "fifteen",
									EN:     "Two durations are given — the walk and the shuttle's interval. The question names which one it wants.",
									VI:     "Có hai khoảng thời gian — thời gian đi bộ và giãn cách xe buýt. Câu hỏi nói rõ nó hỏi cái nào.",
								},
								{
									ID:     "q3",
									Type:   listeningTrueFalse,
									Prompt: "The shuttle bus costs extra.",
									Answer: "false",
									EN:     "\"Free with your conference badge\" is a condition, not a price. A condition attached to something free does not make it cost money.",
									VI:     "\"Free with your conference badge\" là một điều kiện, không phải giá tiền. Điều kiện đi kèm thứ miễn phí không biến nó thành mất phí.",
								},
								{
									ID:     "q4",
									Type:   listeningChoice,
									Prompt: "How is the conference centre described?",
									Options: []map[string]string{
										option("opt_glass", "A glass building directly ahead after the bridge"),
										option("opt_station", "The building beside the station"),
										option("opt_river", "A building on the near side of the river"),
										option("opt_forecourt", "The building on the station forecourt"),
									},
									Answer: "opt_glass",
									EN:     "The description arrives at the end of the route, not the start. Directions are usually built that way.",
									VI:     "Mô tả nằm ở cuối lộ trình, không phải đầu. Phần chỉ đường thường được dựng như vậy.",
								},
							},
							"Directions, with one deliberate trap in them. Three plays exist for exactly this kind of clip.",
							"Phần chỉ đường, có một cái bẫy cố ý. Ba lượt nghe sinh ra là để dành cho loại clip này.",
						),
					},
				},
			},
		},
		{
			Position:    3,
			Title:       "Talks & Discussion (B2)",
			Description: "Short talks where opinion and fact sit in the same sentence.",
			Lessons: []seedLesson{
				{
					Position:         1,
					Title:            "A Short Talk on Habits",
					SkillFocus:       skillListening,
					EstimatedMinutes: 18,
					CEFRLevel:        "b2",
					Activities: []seedActivity{
						listeningItem(
							1,
							"Why Small Habits Work",
							"Most people who fail to build a habit do not fail because they lack discipline. They fail because they set the bar too high on the first day and cannot clear it on the tenth. The research is fairly consistent on this point: a change that feels almost trivially easy is the one most likely to survive a bad week. Reading two pages is not an impressive goal, but it is a goal you can meet when you are tired, and meeting it is what keeps the habit alive. The size of the step matters far less than whether you take it again tomorrow.",
							[]listeningQuestion{
								{
									ID:     "q1",
									Type:   listeningChoice,
									Prompt: "According to the speaker, why do most people fail to build a habit?",
									Options: []map[string]string{
										option("opt_high", "They set an initial goal that is too demanding"),
										option("opt_lazy", "They do not have enough discipline"),
										option("opt_time", "They do not have enough time"),
										option("opt_wrong", "They choose the wrong habit"),
									},
									Answer: "opt_high",
									EN:     "The speaker names a reason in order to reject it, then gives the real one. The rejected reason is the distractor.",
									VI:     "Người nói nêu một lý do để bác bỏ nó, rồi mới đưa ra lý do thật. Lý do bị bác bỏ chính là phương án gây nhiễu.",
								},
								{
									ID:     "q2",
									Type:   listeningTrueFalse,
									Prompt: "The speaker says reading two pages is an impressive goal.",
									Answer: "false",
									EN:     "The clip says the opposite, and then says it is valuable anyway. Both halves of that sentence are the point.",
									VI:     "Clip nói ngược lại, rồi nói rằng dù vậy nó vẫn có giá trị. Cả hai vế của câu đó đều là ý chính.",
								},
								{
									ID:     "q3",
									Type:   listeningChoice,
									Prompt: "What does the speaker say matters most?",
									Options: []map[string]string{
										option("opt_repeat", "Repeating the action the next day"),
										option("opt_size", "The size of each step"),
										option("opt_research", "Following the research exactly"),
										option("opt_early", "Starting early in the day"),
									},
									Answer: "opt_repeat",
									EN:     "The last sentence answers it, and it is phrased as a comparison: one thing matters far less than another.",
									VI:     "Câu cuối trả lời câu hỏi này, và được diễn đạt như một phép so sánh: thứ này quan trọng kém xa thứ kia.",
								},
								{
									ID:     "q4",
									Type:   listeningGapFill,
									Prompt: "A change that survives a bad week is one that feels almost trivially _____.",
									Answer: "easy",
									EN:     "The word is in the middle of a long sentence, which is where a B2 clip tends to put the one that matters.",
									VI:     "Từ này nằm giữa một câu dài — đúng chỗ mà một clip trình độ B2 hay đặt từ quan trọng nhất.",
								},
							},
							"A talk, not an announcement. The speaker states a wrong answer before the right one, on purpose.",
							"Một bài nói, không phải thông báo. Người nói cố ý nêu đáp án sai trước khi nêu đáp án đúng.",
						),
					},
				},
				{
					Position:         2,
					Title:            "Two People Disagreeing",
					SkillFocus:       skillListening,
					EstimatedMinutes: 18,
					CEFRLevel:        "b2",
					Activities: []seedActivity{
						listeningItem(
							1,
							"Working from Home",
							"A: I still think we get more done at home. Nobody interrupts you, and the two hours I used to spend commuting go straight back into the day. B: I don't disagree about the commute. What I'd question is the 'more done' part. I get through my own tasks faster, certainly, but anything that needs three of us takes a week instead of an afternoon. A: That's fair. Maybe the honest answer is that it depends entirely on what kind of work you're doing that week. B: That I can agree with.",
							[]listeningQuestion{
								{
									ID:     "q1",
									Type:   listeningChoice,
									Prompt: "What does speaker B question?",
									Options: []map[string]string{
										option("opt_more", "Whether more work actually gets done at home"),
										option("opt_commute", "Whether commuting wastes time"),
										option("opt_inter", "Whether people are interrupted in the office"),
										option("opt_week", "Whether the week is long enough"),
									},
									Answer: "opt_more",
									EN:     "B agrees with one half of A's point and disputes the other. Which half is stated explicitly — \"I don't disagree about the commute\".",
									VI:     "B đồng ý một nửa ý của A và phản đối nửa còn lại. Nửa nào được nói rõ ràng — \"I don't disagree about the commute\".",
								},
								{
									ID:     "q2",
									Type:   listeningTrueFalse,
									Prompt: "The two speakers end the conversation still disagreeing.",
									Answer: "false",
									EN:     "They land on a shared position in the last two lines. A conversation that begins in disagreement need not end there.",
									VI:     "Họ đi tới một quan điểm chung ở hai câu cuối. Cuộc trò chuyện mở đầu bằng bất đồng không nhất thiết kết thúc như vậy.",
								},
								{
									ID:     "q3",
									Type:   listeningChoice,
									Prompt: "What problem does B describe with working from home?",
									Options: []map[string]string{
										option("opt_group", "Work involving several people takes much longer"),
										option("opt_slow", "Individual tasks take longer"),
										option("opt_tools", "The tools do not work properly"),
										option("opt_hours", "People work longer hours"),
									},
									Answer: "opt_group",
									EN:     "B says individual work is faster and group work is slower, in the same sentence. Only one of those is the complaint.",
									VI:     "B nói việc cá nhân nhanh hơn còn việc nhóm chậm hơn, trong cùng một câu. Chỉ một trong hai là lời phàn nàn.",
								},
								{
									ID:     "q4",
									Type:   listeningGapFill,
									Prompt: "A concludes that the answer depends on what kind of _____ you are doing.",
									Answer: "work",
									EN:     "The concession is the last thing A says, and it is where the conversation actually resolves.",
									VI:     "Lời nhượng bộ là điều cuối cùng A nói, và đó là chỗ cuộc trò chuyện thực sự ngã ngũ.",
								},
							},
							"Two speakers, one clip. Keeping track of who said what is half the exercise.",
							"Hai người nói, một clip. Theo dõi ai nói gì đã là một nửa bài tập.",
						),
					},
				},
			},
		},
	},
}

// The question types the listening grader accepts, named because they are the
// contract between this data, content.QuestionItem and the runner's renderer.
const (
	listeningChoice    = "multiple_choice"
	listeningTrueFalse = "true_false_not_given"
	listeningGapFill   = "gap_fill"

	cfgScript = "script"
	cfgTitle  = "title"
)

// listeningQuestion is one question, authored once. The config and the body are
// derived from it so the two cannot drift: the only difference between them is
// the answer and the explanation, and deriving both from one source is what
// stops a question being asked in the browser that the grader does not know.
type listeningQuestion struct {
	ID      string
	Type    string
	Prompt  string
	Options []map[string]string
	Answer  string
	// EN and VI explain the answer, shown after marking.
	EN string
	VI string
}

// listeningItem builds one listening activity from its script and questions.
func listeningItem(
	position int, title, script string, questions []listeningQuestion, explanationEN, explanationVI string,
) seedActivity {
	configQuestions := make([]map[string]any, 0, len(questions))
	bodyQuestions := make([]map[string]any, 0, len(questions))
	for _, q := range questions {
		shown := map[string]any{
			"id":     q.ID,
			"type":   q.Type,
			"prompt": q.Prompt,
		}
		if len(q.Options) > 0 {
			shown[cfgOptions] = q.Options
		}
		configQuestions = append(configQuestions, shown)

		answered := map[string]any{
			"id":     q.ID,
			"type":   q.Type,
			"prompt": q.Prompt,
			"answer": q.Answer,
			bodyKeyExplanation: map[string]string{
				"text":    q.EN,
				"text_vi": q.VI,
			},
		}
		if len(q.Options) > 0 {
			answered[cfgOptions] = q.Options
		}
		bodyQuestions = append(bodyQuestions, answered)
	}

	return seedActivity{
		Position: position,
		Kind:     kindListeningComprehension,
		// No `script`: the config is what the browser receives, and a script the
		// learner can read turns a listening item into a reading item.
		Config: map[string]any{
			cfgTitle:    title,
			"questions": configQuestions,
		},
		Body: map[string]any{
			cfgTitle:    title,
			cfgScript:   script,
			"questions": bodyQuestions,
			bodyKeyExplanation: map[string]string{
				"text":    explanationEN,
				"text_vi": explanationVI,
			},
		},
	}
}
