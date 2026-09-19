package main

// speakingCourseSeedData defines the 6 speaking lessons across A2–B2.
//
// It exists because the practice hub offered Reading and Writing and nothing for
// speaking, while every `speaking_task` in the database lived in `pool-exam` or
// `pool-placement` — content the exam and placement machinery draws from, which
// no learner can open on purpose. The grading pipeline was built and had nothing
// in front of it.
//
// Two task types, because the grader scores them differently
// (internal/modules/speaking/domain):
//
//   - `read_aloud` carries a `reference_text`. Word accuracy is computed in Go
//     against it with a Levenshtein distance, and it is 70% of the score, so the
//     reference has to be something a learner at that level can actually say.
//   - `respond` carries only a prompt. There is nothing to measure against, so
//     the model's judgement of the transcript stands alone.
//
// Each activity is authored twice over, as the writing course is: `Config` is
// what the lesson runner reads in the browser, `Body` is the content version the
// grader loads. `correct_answer` in the body is the model answer — for a
// read-aloud it is the sentence itself, because saying it is the task.
var speakingCourseSeedData = seedCourse{
	Slug:           "speaking-practice",
	Title:          "Speaking Skills: A2–B2 Practice",
	Description:    "Read-aloud and open speaking tasks with automatic transcription, word accuracy, speaking rate and AI coaching. Pronunciation is not assessed.",
	CEFRFrom:       "A2",
	CEFRTo:         "B2",
	EstimatedHours: 8,
	Units: []seedUnit{
		{
			Position:    1,
			Title:       "Everyday Speaking (A2)",
			Description: "Short sentences read aloud, and simple questions about your own life.",
			Lessons: []seedLesson{
				{
					Position:         1,
					Title:            "Reading Everyday Sentences Aloud",
					SkillFocus:       skillSpeaking,
					EstimatedMinutes: 10,
					CEFRLevel:        "a2",
					Activities: []seedActivity{
						speakingReadAloud(
							1,
							"Read this sentence aloud, clearly and at a comfortable pace.",
							"I usually take the bus to work because it is cheaper than driving.",
							30,
							"A short everyday sentence. Read every word; the score counts words you skipped, added or changed, not how your accent sounds.",
							"Một câu đời thường ngắn. Hãy đọc đủ mọi từ; điểm tính theo từ bị thiếu, thừa hay đọc sai, không tính giọng của bạn.",
						),
						speakingReadAloud(
							2,
							"Read this short exchange aloud. Keep going to the end even if you stumble.",
							"Excuse me, could you tell me where the nearest post office is? It is about ten minutes from here, just past the market.",
							40,
							"Two sentences, one question and one answer. Stopping to restart costs you time but not accuracy — the transcript is what is measured.",
							"Hai câu, một hỏi một đáp. Dừng lại đọc lại sẽ tốn thời gian nhưng không mất điểm chính xác — thứ được đo là bản ghi lời nói.",
						),
					},
				},
				{
					Position:         2,
					Title:            "Talking About Yourself",
					SkillFocus:       skillSpeaking,
					EstimatedMinutes: 12,
					CEFRLevel:        "a2",
					Activities: []seedActivity{
						speakingRespond(
							1,
							"Describe your typical morning. Mention what time you wake up, what you eat, and how you get to work or school.",
							45,
							"I usually wake up at half past six. I have a bowl of noodles and a cup of coffee, and then I take the bus to work. The journey is about thirty minutes, so I listen to music on the way.",
							"Three details were asked for: the time, the food, and the journey. Answering all three matters more than long sentences at this level.",
							"Đề yêu cầu ba chi tiết: giờ dậy, món ăn và cách di chuyển. Ở trình độ này, trả lời đủ ba ý quan trọng hơn câu dài.",
						),
						speakingRespond(
							2,
							"Talk about a place you like going to at the weekend. Say where it is, who you go with, and why you like it.",
							45,
							"I like going to the park near my house at the weekend. It is about fifteen minutes away on foot. I usually go with my younger sister, and we sit under the trees and talk. I like it because it is quiet and it does not cost anything.",
							"Where, who, and why. A reason — even a simple one — is what turns a list into an answer.",
							"Ở đâu, với ai, và vì sao. Một lý do, dù đơn giản, là thứ biến một danh sách thành câu trả lời.",
						),
					},
				},
			},
		},
		{
			Position:    2,
			Title:       "Describing and Explaining (B1)",
			Description: "Longer texts read aloud, and answers that need an example or a reason.",
			Lessons: []seedLesson{
				{
					Position:         1,
					Title:            "Reading an Announcement Aloud",
					SkillFocus:       skillSpeaking,
					EstimatedMinutes: 12,
					CEFRLevel:        "b1",
					Activities: []seedActivity{
						speakingReadAloud(
							1,
							"Read this announcement aloud as if you were speaking to a room of people.",
							"Attention please. The two o'clock train to Manchester has been delayed by approximately twenty minutes. Passengers waiting on platform four should remain where they are, and we will announce the new departure time shortly. We apologise for the inconvenience.",
							60,
							"Numbers and place names are where read-aloud scores usually fall: they are the words a transcript most often gets wrong when they are rushed.",
							"Con số và tên địa danh là chỗ điểm đọc to hay tụt nhất: đó là những từ bản ghi dễ nhận sai nhất khi đọc vội.",
						),
						speakingReadAloud(
							2,
							"Read this paragraph aloud. Pause at the commas rather than racing to the end.",
							"Learning a language is not only about grammar. If you never speak, the words stay on the page, and the moment someone asks you a question you find that you cannot answer it. Speaking badly is the fastest way to start speaking well.",
							60,
							"Longer text, same measure. Speaking rate is reported beside the score: much faster than 150 words a minute usually means words are being dropped.",
							"Văn bản dài hơn, cách đo không đổi. Tốc độ nói hiện cạnh điểm: nhanh hơn khoảng 150 từ/phút thường là dấu hiệu đang nuốt từ.",
						),
					},
				},
				{
					Position:         2,
					Title:            "Describing a Routine and a Photo",
					SkillFocus:       skillSpeaking,
					EstimatedMinutes: 15,
					CEFRLevel:        "b1",
					Activities: []seedActivity{
						speakingRespond(
							1,
							"Describe something you do every week that you would not want to give up. Explain what it is, how long you have done it, and what you would miss about it.",
							60,
							"Every Sunday morning I play football with the same group of friends. We have been doing it for about four years now, since we all worked at the same company. If I stopped, I would miss the match itself much less than the hour afterwards, when we sit and argue about it over breakfast.",
							"The last part of the prompt — what you would miss — is the one candidates skip. An answer that covers two of three questions is an incomplete answer however fluent it sounds.",
							"Phần cuối của đề — điều bạn sẽ nhớ — là phần hay bị bỏ qua nhất. Trả lời hai trên ba ý vẫn là trả lời thiếu, dù nói trôi chảy đến đâu.",
						),
						speakingRespond(
							2,
							"Some people prefer to study in the morning, others late at night. Which do you prefer, and why? Give one example from your own experience.",
							60,
							"I much prefer studying early in the morning. My head is clearest before about ten o'clock, and nothing has happened yet that I need to think about. Last year I tried revising for an exam after work instead, and I read the same page four times without remembering any of it.",
							"A preference, a reason, and an example. The example is what the criteria call task response: without it the answer is an opinion rather than a reply.",
							"Một lựa chọn, một lý do, một ví dụ. Ví dụ chính là thứ tiêu chí gọi là đáp ứng đề bài: thiếu nó thì đó là ý kiến, chưa phải câu trả lời.",
						),
					},
				},
			},
		},
		{
			Position:    3,
			Title:       "Opinion and Argument (B2)",
			Description: "Dense text read aloud, and answers that have to hold a position.",
			Lessons: []seedLesson{
				{
					Position:         1,
					Title:            "Reading a News Paragraph Aloud",
					SkillFocus:       skillSpeaking,
					EstimatedMinutes: 12,
					CEFRLevel:        "b2",
					Activities: []seedActivity{
						speakingReadAloud(
							1,
							"Read this paragraph aloud at the pace of a news broadcast.",
							"The city council has approved a proposal to close the high street to private vehicles between eight in the morning and six in the evening. Shopkeepers have argued that the restriction will cost them customers, while residents point to a marked fall in traffic noise during last summer's trial.",
							75,
							"Subordinate clauses are where a read-aloud comes apart: the sentence is long enough that losing your place costs several words at once.",
							"Mệnh đề phụ là chỗ bài đọc to hay vỡ: câu đủ dài để mất dòng một cái là mất luôn vài từ.",
						),
						speakingReadAloud(
							2,
							"Read this aloud. It contains figures — say them exactly as written.",
							"Between 2019 and 2024 the number of people cycling to work in the city rose by 38 percent, from roughly 12,000 to just over 16,500. Cycling still accounts for less than one journey in ten.",
							60,
							"Figures read aloud are transcribed as words, so \"sixteen thousand five hundred\" and \"16,500\" both count. Saying only \"sixteen thousand\" does not.",
							"Con số đọc to sẽ được ghi thành chữ, nên \"sixteen thousand five hundred\" và \"16,500\" đều được tính. Chỉ nói \"sixteen thousand\" thì không.",
						),
					},
				},
				{
					Position:         2,
					Title:            "Giving and Defending an Opinion",
					SkillFocus:       skillSpeaking,
					EstimatedMinutes: 15,
					CEFRLevel:        "b2",
					Activities: []seedActivity{
						speakingRespond(
							1,
							"Some argue that working from home has made people less productive, others that it has made them more so. Take one side, give two reasons, and acknowledge one point the other side would make.",
							90,
							"I would argue that working from home has made most people more productive, for two reasons. The first is simply time: an hour of commuting returned to the day is an hour that goes somewhere. The second is that the work people do alone — writing, reading, thinking — is exactly the work an office interrupts. I accept, though, that the case against is not trivial: the conversations that happen by accident in an office are hard to schedule, and teams that never meet do lose something real.",
							"Two reasons and a concession. Conceding a point is not weakness at B2 — refusing to acknowledge the other side is what marks an answer as under-developed.",
							"Hai lý do và một điểm nhượng bộ. Ở B2, thừa nhận ý của phía bên kia không phải là yếu — từ chối thừa nhận mới là dấu hiệu lập luận chưa đủ sâu.",
						),
						speakingRespond(
							2,
							"Describe a decision you made that turned out to be wrong. Explain what you decided, why it seemed right at the time, and what you would do differently.",
							90,
							"A few years ago I turned down a job because it paid slightly less than the one I had. At the time that looked like the only sensible reading: the figures were there in front of me and everything else was a guess. What I had not weighed at all was who I would be working with, and I spent the next two years in a role I had stopped learning anything from. I would still compare the salaries, but I would now treat them as one number among several rather than the deciding one.",
							"Past narrative, then a counterfactual. The grammar criterion is watching the tense shift as much as the content.",
							"Kể chuyện quá khứ, rồi giả định. Tiêu chí ngữ pháp chấm cả việc bạn chuyển thì có chuẩn không, chứ không chỉ nội dung.",
						),
					},
				},
			},
		},
	},
}

// speakingReadAloud builds a read-aloud activity.
//
// The reference text is written into `Config` for the runner, `Body` for the
// grader, and `correct_answer` as the model answer — for this task type they are
// the same string three times, because reading the sentence is the task.
func speakingReadAloud(
	position int, prompt, referenceText string, seconds int, explanationEN, explanationVI string,
) seedActivity {
	return seedActivity{
		Position: position,
		Kind:     kindSpeakingTask,
		Config: map[string]any{
			cfgTaskType:      taskTypeReadAloud,
			bodyKeyPrompt:    prompt,
			cfgReferenceText: referenceText,
			cfgSpeakingTime:  seconds,
		},
		Body: map[string]any{
			cfgTaskType:          taskTypeReadAloud,
			bodyKeyPrompt:        prompt,
			cfgReferenceText:     referenceText,
			cfgSpeakingTime:      seconds,
			bodyKeyCorrectAnswer: referenceText,
			bodyKeyExplanation: map[string]string{
				"text":    explanationEN,
				"text_vi": explanationVI,
			},
		},
	}
}

// speakingRespond builds an open speaking activity.
//
// There is no reference text, so nothing is measured in Go and the model's
// reading of the transcript carries the score. `correct_answer` is a model
// answer the learner can compare against, not something they are marked against.
func speakingRespond(
	position int, prompt string, seconds int, modelAnswer, explanationEN, explanationVI string,
) seedActivity {
	return seedActivity{
		Position: position,
		Kind:     kindSpeakingTask,
		Config: map[string]any{
			cfgTaskType:     taskTypeRespond,
			bodyKeyPrompt:   prompt,
			cfgSpeakingTime: seconds,
			cfgSampleAnswer: modelAnswer,
		},
		Body: map[string]any{
			cfgTaskType:          taskTypeRespond,
			bodyKeyPrompt:        prompt,
			cfgSpeakingTime:      seconds,
			cfgSampleAnswer:      modelAnswer,
			bodyKeyCorrectAnswer: modelAnswer,
			bodyKeyExplanation: map[string]string{
				"text":    explanationEN,
				"text_vi": explanationVI,
			},
		},
	}
}
