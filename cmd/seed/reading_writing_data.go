package main

// readingCourseSeedData defines the 6 reading comprehension lessons across A2-B2.
var readingCourseSeedData = seedCourse{
	Slug:           "reading-practice",
	Title:          "Reading Comprehension: A2–B2",
	Description:    "Curated reading passages from A2 to B2 with comprehension questions and reading speed measurement.",
	CEFRFrom:       "A2",
	CEFRTo:         "B2",
	EstimatedHours: 12,
	Units: []seedUnit{
		{
			Position:    1,
			Title:       "Everyday Topics & Announcements (A2)",
			Description: "Short descriptive and informational texts about daily routines and leisure activities.",
			Lessons: []seedLesson{
				{
					Position:         1,
					Title:            "Planning a Weekend Trip",
					SkillFocus:       skillReading,
					EstimatedMinutes: 15,
					Activities: []seedActivity{
						{
							Position: 1,
							Kind:     kindReadingComprehension,
							Config: map[string]any{
								cfgPassageTitle: "Planning a Weekend Trip",
								cfgPassage: "My friends and I have decided to spend next weekend in the countryside. We want to take a break from the noise and busy schedule of the city. We are leaving early on Saturday morning at 7:30 AM by train so that we can arrive before noon. We found a charming countryside inn near a forested hill. The hotel reservation includes a complimentary hot breakfast every morning. During our two-day stay, we plan to go hiking along the pine trails, take photos of the wildlife, and enjoy dinner together at a family-run tavern in the nearby village. We are all looking forward to recharging our energy before the work week begins.",
								"questions": []map[string]any{
									{
										"id":     "q1",
										"type":   "multiple_choice",
										"prompt": "What is the primary purpose of their weekend trip?",
										"options": []map[string]string{
											option("opt_relax", "To relax and take a break from city life in the countryside"),
											option("opt_biz", "To attend a business conference in another town"),
											option("opt_shop", "To go shopping in a commercial district"),
											option("opt_family", "To visit distant relatives living in the village"),
										},
									},
									{
										"id":     "q2",
										"type":   "true_false_not_given",
										"prompt": "The group will travel to the countryside by train.",
									},
									{
										"id":     "q3",
										"type":   "gap_fill",
										"prompt": "The hotel reservation includes a complimentary hot _____ every morning.",
									},
									{
										"id":     "q4",
										"type":   "multiple_choice",
										"prompt": "What time does the train leave on Saturday morning?",
										"options": []map[string]string{
											option("opt_t730", "7:30 AM"),
											option("opt_t900", "9:00 AM"),
											option("opt_t1100", "11:00 AM"),
											option("opt_t600", "6:00 AM"),
										},
									},
								},
							},
							Body: map[string]any{
								cfgPassageTitle: "Planning a Weekend Trip",
								cfgPassage:      "My friends and I have decided to spend next weekend in the countryside. We want to take a break from the noise and busy schedule of the city. We are leaving early on Saturday morning at 7:30 AM by train so that we can arrive before noon. We found a charming countryside inn near a forested hill. The hotel reservation includes a complimentary hot breakfast every morning. During our two-day stay, we plan to go hiking along the pine trails, take photos of the wildlife, and enjoy dinner together at a family-run tavern in the nearby village. We are all looking forward to recharging our energy before the work week begins.",
								"questions": []map[string]any{
									{
										"id":     "q1",
										"type":   "multiple_choice",
										"prompt": "What is the primary purpose of their weekend trip?",
										"options": []map[string]string{
											option("opt_relax", "To relax and take a break from city life in the countryside"),
											option("opt_biz", "To attend a business conference in another town"),
											option("opt_shop", "To go shopping in a commercial district"),
											option("opt_family", "To visit distant relatives living in the village"),
										},
										"answer": "opt_relax",
									},
									{
										"id":     "q2",
										"type":   "true_false_not_given",
										"prompt": "The group will travel to the countryside by train.",
										"answer": "true",
									},
									{
										"id":         "q3",
										"type":       "gap_fill",
										"prompt":     "The hotel reservation includes a complimentary hot _____ every morning.",
										"answer":     "breakfast",
										"acceptable": acceptable("breakfast", "Breakfast"),
									},
									{
										"id":     "q4",
										"type":   "multiple_choice",
										"prompt": "What time does the train leave on Saturday morning?",
										"options": []map[string]string{
											option("opt_t730", "7:30 AM"),
											option("opt_t900", "9:00 AM"),
											option("opt_t1100", "11:00 AM"),
											option("opt_t600", "6:00 AM"),
										},
										"answer": "opt_t730",
									},
								},
								bodyKeyExplanation: map[string]string{
									"text":    "The group travels by train at 7:30 AM to relax in the countryside, and the inn offers complimentary breakfast.",
									"text_vi": "Nhóm đi tàu lúc 7:30 sáng để nghỉ ngơi ở vùng quê và nhà nghỉ có phục vụ bữa sáng miễn phí.",
								},
							},
						},
					},
				},
				{
					Position:         2,
					Title:            "A Healthy Morning Routine",
					SkillFocus:       skillReading,
					EstimatedMinutes: 15,
					Activities: []seedActivity{
						{
							Position: 1,
							Kind:     kindReadingComprehension,
							Config: map[string]any{
								cfgPassageTitle: "A Healthy Morning Routine",
								cfgPassage: "Maya firmly believes that the way she starts her morning determines how productive and calm she feels throughout the entire day. She wakes up at 6:00 AM and immediately drinks a full glass of warm lemon water before preparing any food. Afterwards, she spends twenty minutes doing gentle yoga stretches and deep breathing exercises in her living room. For breakfast, Maya always prepares warm oatmeal topped with fresh berries, sliced almonds, and a drizzle of honey. Most importantly, Maya has established a strict rule never to check work emails or social media before she physically arrives at her office at 8:30 AM. This intentional quiet time allows her mind to remain focused and calm.",
								"questions": []map[string]any{
									{
										"id":     "q1",
										"type":   "true_false_not_given",
										"prompt": "Maya drinks warm lemon water before having breakfast.",
									},
									{
										"id":     "q2",
										"type":   "multiple_choice",
										"prompt": "How much time does Maya spend on yoga and breathing exercises?",
										"options": []map[string]string{
											option("opt_y20", "Twenty minutes"),
											option("opt_y45", "Forty-five minutes"),
											option("opt_y60", "One hour"),
											option("opt_y10", "Ten minutes"),
										},
									},
									{
										"id":     "q3",
										"type":   "gap_fill",
										"prompt": "Maya tops her morning oatmeal with almonds, honey, and fresh _____.",
									},
									{
										"id":     "q4",
										"type":   "true_false_not_given",
										"prompt": "Maya reads work messages as soon as she finishes breakfast.",
									},
								},
							},
							Body: map[string]any{
								cfgPassageTitle: "A Healthy Morning Routine",
								cfgPassage:      "Maya firmly believes that the way she starts her morning determines how productive and calm she feels throughout the entire day. She wakes up at 6:00 AM and immediately drinks a full glass of warm lemon water before preparing any food. Afterwards, she spends twenty minutes doing gentle yoga stretches and deep breathing exercises in her living room. For breakfast, Maya always prepares warm oatmeal topped with fresh berries, sliced almonds, and a drizzle of honey. Most importantly, Maya has established a strict rule never to check work emails or social media before she physically arrives at her office at 8:30 AM. This intentional quiet time allows her mind to remain focused and calm.",
								"questions": []map[string]any{
									{
										"id":     "q1",
										"type":   "true_false_not_given",
										"prompt": "Maya drinks warm lemon water before having breakfast.",
										"answer": "true",
									},
									{
										"id":     "q2",
										"type":   "multiple_choice",
										"prompt": "How much time does Maya spend on yoga and breathing exercises?",
										"options": []map[string]string{
											option("opt_y20", "Twenty minutes"),
											option("opt_y45", "Forty-five minutes"),
											option("opt_y60", "One hour"),
											option("opt_y10", "Ten minutes"),
										},
										"answer": "opt_y20",
									},
									{
										"id":         "q3",
										"type":       "gap_fill",
										"prompt":     "Maya tops her morning oatmeal with almonds, honey, and fresh _____.",
										"answer":     "berries",
										"acceptable": acceptable("berries", "Berries"),
									},
									{
										"id":     "q4",
										"type":   "true_false_not_given",
										"prompt": "Maya reads work messages as soon as she finishes breakfast.",
										"answer": "false",
									},
								},
								bodyKeyExplanation: map[string]string{
									"text":    "Maya drinks water first, spends 20 minutes doing yoga, eats berries on oatmeal, and avoids work messages until at the office.",
									"text_vi": "Maya uống nước chanh trước, tập yoga 20 phút, ăn yến mạch với quả mọng và tránh xem email công việc trước khi tới cơ quan.",
								},
							},
						},
					},
				},
			},
		},
		{
			Position:    2,
			Title:       "Society, Work & Environment (B1)",
			Description: "Explanatory and informative articles exploring workplace trends and urban sustainability.",
			Lessons: []seedLesson{
				{
					Position:         1,
					Title:            "The Rise of Remote Work",
					SkillFocus:       skillReading,
					EstimatedMinutes: 20,
					Activities: []seedActivity{
						{
							Position: 1,
							Kind:     kindReadingComprehension,
							Config: map[string]any{
								cfgPassageTitle: "The Rise of Remote Work",
								cfgPassage: "Over the last decade, advances in cloud computing and video conferencing have permanently reshaped how people work. Millions of employees across diverse industries now perform their daily tasks from home offices rather than corporate headquarters. Surveys consistently show that remote workers appreciate the elimination of lengthy commutes and the flexibility to schedule their work around family commitments. However, working remotely also presents significant challenges. Many individuals report feeling isolated from colleagues, and the boundary separating professional obligations from personal life often becomes blurred. Successful remote companies address this by hosting structured virtual check-ins and promoting asynchronous communication so team members do not feel pressured to respond to messages after hours.",
								"questions": []map[string]any{
									{
										"id":     "q1",
										"type":   "multiple_choice",
										"prompt": "What major advantage do remote employees consistently report?",
										"options": []map[string]string{
											option("opt_rw_flex", "Elimination of daily commutes and greater schedule flexibility"),
											option("opt_rw_pay", "Substantially higher guaranteed baseline salaries"),
											option("opt_rw_vac", "Unlimited company-funded annual vacation days"),
											option("opt_rw_soc", "Frequent in-person team social dinners"),
										},
									},
									{
										"id":     "q2",
										"type":   "true_false_not_given",
										"prompt": "Working from home can make it difficult to separate professional life from personal time.",
									},
									{
										"id":     "q3",
										"type":   "gap_fill",
										"prompt": "Successful remote companies use asynchronous _____ to avoid pressuring workers after hours.",
									},
									{
										"id":     "q4",
										"type":   "true_false_not_given",
										"prompt": "All remote workers are required by law to work 40 hours per week.",
									},
									{
										"id":     "q5",
										"type":   "multiple_choice",
										"prompt": "How do effective companies combat worker isolation in remote settings?",
										"options": []map[string]string{
											option("opt_rw_check", "By scheduling regular virtual check-ins"),
											option("opt_rw_cam", "By monitoring employees through webcams all day"),
											option("opt_rw_comp", "By requiring daily travel to the nearest office branch"),
											option("opt_rw_pen", "By penalizing staff who work from home on Fridays"),
										},
									},
								},
							},
							Body: map[string]any{
								cfgPassageTitle: "The Rise of Remote Work",
								cfgPassage:      "Over the last decade, advances in cloud computing and video conferencing have permanently reshaped how people work. Millions of employees across diverse industries now perform their daily tasks from home offices rather than corporate headquarters. Surveys consistently show that remote workers appreciate the elimination of lengthy commutes and the flexibility to schedule their work around family commitments. However, working remotely also presents significant challenges. Many individuals report feeling isolated from colleagues, and the boundary separating professional obligations from personal life often becomes blurred. Successful remote companies address this by hosting structured virtual check-ins and promoting asynchronous communication so team members do not feel pressured to respond to messages after hours.",
								"questions": []map[string]any{
									{
										"id":     "q1",
										"type":   "multiple_choice",
										"prompt": "What major advantage do remote employees consistently report?",
										"options": []map[string]string{
											option("opt_rw_flex", "Elimination of daily commutes and greater schedule flexibility"),
											option("opt_rw_pay", "Substantially higher guaranteed baseline salaries"),
											option("opt_rw_vac", "Unlimited company-funded annual vacation days"),
											option("opt_rw_soc", "Frequent in-person team social dinners"),
										},
										"answer": "opt_rw_flex",
									},
									{
										"id":     "q2",
										"type":   "true_false_not_given",
										"prompt": "Working from home can make it difficult to separate professional life from personal time.",
										"answer": "true",
									},
									{
										"id":         "q3",
										"type":       "gap_fill",
										"prompt":     "Successful remote companies use asynchronous _____ to avoid pressuring workers after hours.",
										"answer":     "communication",
										"acceptable": acceptable("communication", "Communication"),
									},
									{
										"id":     "q4",
										"type":   "true_false_not_given",
										"prompt": "All remote workers are required by law to work 40 hours per week.",
										"answer": "not_given",
									},
									{
										"id":     "q5",
										"type":   "multiple_choice",
										"prompt": "How do effective companies combat worker isolation in remote settings?",
										"options": []map[string]string{
											option("opt_rw_check", "By scheduling regular virtual check-ins"),
											option("opt_rw_cam", "By monitoring employees through webcams all day"),
											option("opt_rw_comp", "By requiring daily travel to the nearest office branch"),
											option("opt_rw_pen", "By penalizing staff who work from home on Fridays"),
										},
										"answer": "opt_rw_check",
									},
								},
								bodyKeyExplanation: map[string]string{
									"text":    "Remote work provides flexibility and eliminates commutes, while virtual check-ins and asynchronous communication help manage isolation.",
									"text_vi": "Làm việc từ xa đem lại sự linh hoạt và giảm thời gian đi lại, trong khi các buổi họp trực tuyến và trao đổi không đồng bộ giúp hạn chế sự cô lập.",
								},
							},
						},
					},
				},
				{
					Position:         2,
					Title:            "Urban Agriculture and Green Spaces",
					SkillFocus:       skillReading,
					EstimatedMinutes: 20,
					Activities: []seedActivity{
						{
							Position: 1,
							Kind:     kindReadingComprehension,
							Config: map[string]any{
								cfgPassageTitle: "Urban Agriculture and Green Spaces",
								cfgPassage: "As metropolitan populations continue to swell, city planners are transforming neglected vacant lots and unused residential rooftops into thriving agricultural spaces. These community gardens yield nutritious organic produce while fostering social cohesion among diverse neighbors who work together to cultivate crops. Ecologically, rooftop gardens and vertical farms serve an essential purpose by absorbing solar radiation and lowering ambient temperatures, which helps combat the intense urban heat island effect that plagues dense concrete cities. In addition, cultivating vegetables locally dramatically reduces the transportation emissions normally required to haul food from distant rural farms into urban supermarkets.",
								"questions": []map[string]any{
									{
										"id":     "q1",
										"type":   "multiple_choice",
										"prompt": "Where are urban community gardens commonly established?",
										"options": []map[string]string{
											option("opt_ug_roof", "On abandoned vacant lots and building rooftops"),
											option("opt_ug_sub", "Inside underground metro subway stations"),
											option("opt_ug_high", "Along high-speed interstate highway lanes"),
											option("opt_ug_airport", "Across commercial airport runways"),
										},
									},
									{
										"id":     "q2",
										"type":   "gap_fill",
										"prompt": "Rooftop gardens help combat the urban heat _____ effect by absorbing solar heat.",
									},
									{
										"id":     "q3",
										"type":   "true_false_not_given",
										"prompt": "Community gardens reduce social interaction among neighborhood residents.",
									},
									{
										"id":     "q4",
										"type":   "true_false_not_given",
										"prompt": "Growing food locally decreases the transportation emissions associated with food distribution.",
									},
									{
										"id":     "q5",
										"type":   "multiple_choice",
										"prompt": "What ecological advantage do urban gardens provide?",
										"options": []map[string]string{
											option("opt_ug_cool", "They absorb solar radiation and lower local temperatures"),
											option("opt_ug_rain", "They completely eliminate seasonal rainfall"),
											option("opt_ug_wind", "They stop all strong oceanic wind currents"),
											option("opt_ug_snow", "They guarantee sub-zero snowfall throughout winter"),
										},
									},
								},
							},
							Body: map[string]any{
								cfgPassageTitle: "Urban Agriculture and Green Spaces",
								cfgPassage:      "As metropolitan populations continue to swell, city planners are transforming neglected vacant lots and unused residential rooftops into thriving agricultural spaces. These community gardens yield nutritious organic produce while fostering social cohesion among diverse neighbors who work together to cultivate crops. Ecologically, rooftop gardens and vertical farms serve an essential purpose by absorbing solar radiation and lowering ambient temperatures, which helps combat the intense urban heat island effect that plagues dense concrete cities. In addition, cultivating vegetables locally dramatically reduces the transportation emissions normally required to haul food from distant rural farms into urban supermarkets.",
								"questions": []map[string]any{
									{
										"id":     "q1",
										"type":   "multiple_choice",
										"prompt": "Where are urban community gardens commonly established?",
										"options": []map[string]string{
											option("opt_ug_roof", "On abandoned vacant lots and building rooftops"),
											option("opt_ug_sub", "Inside underground metro subway stations"),
											option("opt_ug_high", "Along high-speed interstate highway lanes"),
											option("opt_ug_airport", "Across commercial airport runways"),
										},
										"answer": "opt_ug_roof",
									},
									{
										"id":         "q2",
										"type":       "gap_fill",
										"prompt":     "Rooftop gardens help combat the urban heat _____ effect by absorbing solar heat.",
										"answer":     "island",
										"acceptable": acceptable("island", "Island"),
									},
									{
										"id":     "q3",
										"type":   "true_false_not_given",
										"prompt": "Community gardens reduce social interaction among neighborhood residents.",
										"answer": "false",
									},
									{
										"id":     "q4",
										"type":   "true_false_not_given",
										"prompt": "Growing food locally decreases the transportation emissions associated with food distribution.",
										"answer": "true",
									},
									{
										"id":     "q5",
										"type":   "multiple_choice",
										"prompt": "What ecological advantage do urban gardens provide?",
										"options": []map[string]string{
											option("opt_ug_cool", "They absorb solar radiation and lower local temperatures"),
											option("opt_ug_rain", "They completely eliminate seasonal rainfall"),
											option("opt_ug_wind", "They stop all strong oceanic wind currents"),
											option("opt_ug_snow", "They guarantee sub-zero snowfall throughout winter"),
										},
										"answer": "opt_ug_cool",
									},
								},
								bodyKeyExplanation: map[string]string{
									"text":    "Urban gardens reduce the heat island effect, cut transportation emissions, and strengthen community ties.",
									"text_vi": "Vườn đô thị giúp giảm hiệu ứng đảo nhiệt, cắt giảm khí thải vận chuyển và củng cố tinh thần cộng đồng.",
								},
							},
						},
					},
				},
			},
		},
		{
			Position:    3,
			Title:       "Science, Technology & Human Behavior (B2)",
			Description: "In-depth analytical essays on cognitive psychology and energy transition.",
			Lessons: []seedLesson{
				{
					Position:         1,
					Title:            "The Science of Decision Fatigue",
					SkillFocus:       skillReading,
					EstimatedMinutes: 25,
					Activities: []seedActivity{
						{
							Position: 1,
							Kind:     kindReadingComprehension,
							Config: map[string]any{
								cfgPassageTitle: "The Science of Decision Fatigue",
								cfgPassage: "Human willpower is not an inexhaustible character trait, but rather a finite mental resource that depletes with continuous use. Every decision made throughout the day—from selecting what clothes to wear to negotiating complex business contracts—consumes glucose and mental stamina. Psychologists term the resulting cognitive decline 'decision fatigue'. When individuals become mentally drained, their capacity for critical reasoning deteriorates markedly, leading them to either act impulsively or postpone tough judgments by defaulting to the easiest option. A landmark study evaluating judicial parole decisions demonstrated that judges granted parole at significantly higher rates immediately following morning and afternoon food breaks. As cognitive reserves dwindled before lunch, the likelihood of a prisoner being granted parole plummeted to near zero, as refusing parole represented the safer default decision.",
								"questions": []map[string]any{
									{
										"id":     "q1",
										"type":   "multiple_choice",
										"prompt": "What is the psychological definition of 'decision fatigue'?",
										"options": []map[string]string{
											option("opt_df_def", "The decline in decision quality caused by the depletion of mental stamina"),
											option("opt_df_phys", "Physical muscle tiredness resulting from aerobic exercise"),
											option("opt_df_amnesia", "Total memory loss regarding past personal events"),
											option("opt_df_anxiety", "A chronic phobia of speaking in public spaces"),
										},
									},
									{
										"id":     "q2",
										"type":   "true_false_not_given",
										"prompt": "Trivial daily decisions consume the same cognitive reserves as momentous professional choices.",
									},
									{
										"id":     "q3",
										"type":   "gap_fill",
										"prompt": "When decision reserves run low, people tend to default to the _____ option.",
									},
									{
										"id":     "q4",
										"type":   "multiple_choice",
										"prompt": "What did the study reveal about judicial parole rulings?",
										"options": []map[string]string{
											option("opt_df_judge", "Parole approval rates were substantially higher right after food breaks"),
											option("opt_df_equal", "Parole was granted evenly across all hours of the day"),
											option("opt_df_harsh", "Judges became much harsher immediately after their lunch meal"),
											option("opt_df_fast", "Hearings took half as long when judges were tired"),
										},
									},
									{
										"id":     "q5",
										"type":   "true_false_not_given",
										"prompt": "Glucose intake has no correlation with replenishing willpower resources.",
									},
									{
										"id":     "q6",
										"type":   "gap_fill",
										"prompt": "Denying parole was observed to be the safer _____ choice for fatigued judges.",
									},
								},
							},
							Body: map[string]any{
								cfgPassageTitle: "The Science of Decision Fatigue",
								cfgPassage:      "Human willpower is not an inexhaustible character trait, but rather a finite mental resource that depletes with continuous use. Every decision made throughout the day—from selecting what clothes to wear to negotiating complex business contracts—consumes glucose and mental stamina. Psychologists term the resulting cognitive decline 'decision fatigue'. When individuals become mentally drained, their capacity for critical reasoning deteriorates markedly, leading them to either act impulsively or postpone tough judgments by defaulting to the easiest option. A landmark study evaluating judicial parole decisions demonstrated that judges granted parole at significantly higher rates immediately following morning and afternoon food breaks. As cognitive reserves dwindled before lunch, the likelihood of a prisoner being granted parole plummeted to near zero, as refusing parole represented the safer default decision.",
								"questions": []map[string]any{
									{
										"id":     "q1",
										"type":   "multiple_choice",
										"prompt": "What is the psychological definition of 'decision fatigue'?",
										"options": []map[string]string{
											option("opt_df_def", "The decline in decision quality caused by the depletion of mental stamina"),
											option("opt_df_phys", "Physical muscle tiredness resulting from aerobic exercise"),
											option("opt_df_amnesia", "Total memory loss regarding past personal events"),
											option("opt_df_anxiety", "A chronic phobia of speaking in public spaces"),
										},
										"answer": "opt_df_def",
									},
									{
										"id":     "q2",
										"type":   "true_false_not_given",
										"prompt": "Trivial daily decisions consume the same cognitive reserves as momentous professional choices.",
										"answer": "true",
									},
									{
										"id":         "q3",
										"type":       "gap_fill",
										"prompt":     "When decision reserves run low, people tend to default to the _____ option.",
										"answer":     "easiest",
										"acceptable": acceptable("easiest", "Easiest", "default"),
									},
									{
										"id":     "q4",
										"type":   "multiple_choice",
										"prompt": "What did the study reveal about judicial parole rulings?",
										"options": []map[string]string{
											option("opt_df_judge", "Parole approval rates were substantially higher right after food breaks"),
											option("opt_df_equal", "Parole was granted evenly across all hours of the day"),
											option("opt_df_harsh", "Judges became much harsher immediately after their lunch meal"),
											option("opt_df_fast", "Hearings took half as long when judges were tired"),
										},
										"answer": "opt_df_judge",
									},
									{
										"id":     "q5",
										"type":   "true_false_not_given",
										"prompt": "Glucose intake has no correlation with replenishing willpower resources.",
										"answer": "false",
									},
									{
										"id":         "q6",
										"type":       "gap_fill",
										"prompt":     "Denying parole was observed to be the safer _____ choice for fatigued judges.",
										"answer":     "default",
										"acceptable": acceptable("default", "Default"),
									},
								},
								bodyKeyExplanation: map[string]string{
									"text":    "Decision fatigue depletes mental stamina and leads to defaulting on easy options, as seen in judicial parole variances.",
									"text_vi": "Kiệt sức vì ra quyết định làm cạn kiệt ý chí dẫn đến xu hướng chọn phương án dễ nhất, minh chứng qua tỉ lệ ân xá của thẩm phán sau giờ ăn.",
								},
							},
						},
					},
				},
				{
					Position:         2,
					Title:            "The Transition to Smart Grids",
					SkillFocus:       skillReading,
					EstimatedMinutes: 25,
					Activities: []seedActivity{
						{
							Position: 1,
							Kind:     kindReadingComprehension,
							Config: map[string]any{
								cfgPassageTitle: "The Transition to Smart Grids",
								cfgPassage: "Decarbonizing global energy infrastructure hinges on successfully modernizing traditional electrical grids into intelligent, bidirectional networks known as smart grids. Conventional power systems were designed around centralized fossil-fuel power plants that generated electricity and dispatched it along a one-way pipeline to passive consumers. In contrast, modern renewable energy sources such as solar photovoltaics and wind turbines produce variable amounts of electricity that fluctuate unpredictably depending on weather patterns. Smart grids incorporate automated sensors, Internet-of-Things telecommunications, and machine-learning forecasting algorithms to dynamically match real-time electricity supply with consumer demand. Furthermore, grid-scale battery storage facilities store excess clean energy generated during sunny or windy intervals and inject it back into the transmission lines during peak evening hours, ensuring stability without firing up carbon-intensive coal generators.",
								"questions": []map[string]any{
									{
										"id":     "q1",
										"type":   "multiple_choice",
										"prompt": "What was the structural design of traditional electrical power networks?",
										"options": []map[string]string{
											option("opt_sg_trad", "Centralized generation with one-way distribution to passive users"),
											option("opt_sg_bi", "Fully bidirectional flows powered entirely by local home batteries"),
											option("opt_sg_sat", "Space-based microwave transmission bypassing power lines"),
											option("opt_sg_hydro", "Sole reliance on run-of-the-river hydroelectric stations"),
										},
									},
									{
										"id":     "q2",
										"type":   "true_false_not_given",
										"prompt": "Solar and wind energy output varies based on prevailing weather patterns.",
									},
									{
										"id":     "q3",
										"type":   "gap_fill",
										"prompt": "Smart grids use digital sensors to match supply with customer _____ in real time.",
									},
									{
										"id":     "q4",
										"type":   "multiple_choice",
										"prompt": "How do grid-scale batteries support electrical stability?",
										"options": []map[string]string{
											option("opt_sg_batt", "They store surplus generation and release it during high peak hours"),
											option("opt_sg_cool", "They cool nuclear fuel rods during unexpected droughts"),
											option("opt_sg_gen", "They create synthetic fossil fuel lubricants"),
											option("opt_sg_cost", "They eliminate all electric company utility bills permanently"),
										},
									},
									{
										"id":     "q5",
										"type":   "true_false_not_given",
										"prompt": "Smart grid technologies eliminate the necessity of any transmission wires.",
									},
									{
										"id":     "q6",
										"type":   "gap_fill",
										"prompt": "Smart grids prevent having to start carbon-intensive _____ generators when demand spikes.",
									},
								},
							},
							Body: map[string]any{
								cfgPassageTitle: "The Transition to Smart Grids",
								cfgPassage:      "Decarbonizing global energy infrastructure hinges on successfully modernizing traditional electrical grids into intelligent, bidirectional networks known as smart grids. Conventional power systems were designed around centralized fossil-fuel power plants that generated electricity and dispatched it along a one-way pipeline to passive consumers. In contrast, modern renewable energy sources such as solar photovoltaics and wind turbines produce variable amounts of electricity that fluctuate unpredictably depending on weather patterns. Smart grids incorporate automated sensors, Internet-of-Things telecommunications, and machine-learning forecasting algorithms to dynamically match real-time electricity supply with consumer demand. Furthermore, grid-scale battery storage facilities store excess clean energy generated during sunny or windy intervals and inject it back into the transmission lines during peak evening hours, ensuring stability without firing up carbon-intensive coal generators.",
								"questions": []map[string]any{
									{
										"id":     "q1",
										"type":   "multiple_choice",
										"prompt": "What was the structural design of traditional electrical power networks?",
										"options": []map[string]string{
											option("opt_sg_trad", "Centralized generation with one-way distribution to passive users"),
											option("opt_sg_bi", "Fully bidirectional flows powered entirely by local home batteries"),
											option("opt_sg_sat", "Space-based microwave transmission bypassing power lines"),
											option("opt_sg_hydro", "Sole reliance on run-of-the-river hydroelectric stations"),
										},
										"answer": "opt_sg_trad",
									},
									{
										"id":     "q2",
										"type":   "true_false_not_given",
										"prompt": "Solar and wind energy output varies based on prevailing weather patterns.",
										"answer": "true",
									},
									{
										"id":         "q3",
										"type":       "gap_fill",
										"prompt":     "Smart grids use digital sensors to match supply with customer _____ in real time.",
										"answer":     "demand",
										"acceptable": acceptable("demand", "Demand"),
									},
									{
										"id":     "q4",
										"type":   "multiple_choice",
										"prompt": "How do grid-scale batteries support electrical stability?",
										"options": []map[string]string{
											option("opt_sg_batt", "They store surplus generation and release it during high peak hours"),
											option("opt_sg_cool", "They cool nuclear fuel rods during unexpected droughts"),
											option("opt_sg_gen", "They create synthetic fossil fuel lubricants"),
											option("opt_sg_cost", "They eliminate all electric company utility bills permanently"),
										},
										"answer": "opt_sg_batt",
									},
									{
										"id":     "q5",
										"type":   "true_false_not_given",
										"prompt": "Smart grid technologies eliminate the necessity of any transmission wires.",
										"answer": "false",
									},
									{
										"id":         "q6",
										"type":       "gap_fill",
										"prompt":     "Smart grids prevent having to start carbon-intensive _____ generators when demand spikes.",
										"answer":     "coal",
										"acceptable": acceptable("coal", "Coal"),
									},
								},
								bodyKeyExplanation: map[string]string{
									"text":    "Smart grids transition from one-way systems to bidirectional networks balancing variable renewable power with batteries and real-time sensors.",
									"text_vi": "Lưới điện thông minh chuyển từ hệ thống truyền tải một chiều sang mạng lưới hai chiều, dùng pin lưu trữ và cảm biến để cân bằng năng lượng tái tạo.",
								},
							},
						},
					},
				},
			},
		},
	},
}

// writingCourseSeedData defines the 6 writing prompt lessons across A2-B2.
var writingCourseSeedData = seedCourse{
	Slug:           "writing-practice",
	Title:          "Writing Skills: A2–B2 Prompts",
	Description:    "Structured writing tasks from emails to academic opinion essays with automated AI grading and model answers.",
	CEFRFrom:       "A2",
	CEFRTo:         "B2",
	EstimatedHours: 12,
	Units: []seedUnit{
		{
			Position:    1,
			Title:       "Informal & Semi-Formal Emails (A2 - 60 words)",
			Description: "Write short communicative messages for invitations, bookings, and inquiries.",
			Lessons: []seedLesson{
				{
					Position:         1,
					Title:            "Inviting a Friend to an Event",
					SkillFocus:       skillWriting,
					EstimatedMinutes: 15,
					Activities: []seedActivity{
						{
							Position: 1,
							Kind:     kindWritingPrompt,
							Config: map[string]any{
								bodyKeyPrompt:   "Write an informal email to your friend Sam inviting them to a housewarming party next weekend. Mention the date and time, the location of your new apartment, and ask them to bring a favorite snack or dessert.",
								cfgRubric:       "Write at least 60 words. Include greeting, party details (time and location), a polite request to bring food, and an informal closing.",
								cfgMinWords:     60,
								cfgSampleAnswer: "Hi Sam,\n\nI hope you are having a wonderful week! I would love to invite you to my housewarming party next Saturday at 6:00 PM. My new apartment is located at 45 Maple Street, right next to the central park. Could you please bring your famous chocolate cookies? I really look forward to catching up soon!\n\nBest,\nAlex",
							},
							Body: map[string]any{
								bodyKeyPrompt:        "Write an informal email to your friend Sam inviting them to a housewarming party next weekend. Mention the date and time, the location of your new apartment, and ask them to bring a favorite snack or dessert.",
								cfgRubric:            "Write at least 60 words. Include greeting, party details (time and location), a polite request to bring food, and an informal closing.",
								cfgMinWords:          60,
								bodyKeyCorrectAnswer: "Hi Sam,\n\nI hope you are having a wonderful week! I would love to invite you to my housewarming party next Saturday at 6:00 PM. My new apartment is located at 45 Maple Street, right next to the central park. Could you please bring your famous chocolate cookies? I really look forward to catching up soon!\n\nBest,\nAlex",
								cfgSampleAnswer:      "Hi Sam,\n\nI hope you are having a wonderful week! I would love to invite you to my housewarming party next Saturday at 6:00 PM. My new apartment is located at 45 Maple Street, right next to the central park. Could you please bring your famous chocolate cookies? I really look forward to catching up soon!\n\nBest,\nAlex",
								bodyKeyExplanation: map[string]string{
									"text":    "An informal email should feature a warm greeting, clear event time and address, a request for a snack, and friendly sign-off.",
									"text_vi": "Email thân mật cần có lời chào ấm áp, thời gian và địa điểm tổ chức rõ ràng, lời nhờ mang đồ ăn kèm và lời chào kết thân thiện.",
								},
							},
						},
					},
				},
				{
					Position:         2,
					Title:            "Inquiring About Hotel Accommodation",
					SkillFocus:       skillWriting,
					EstimatedMinutes: 15,
					Activities: []seedActivity{
						{
							Position: 1,
							Kind:     kindWritingPrompt,
							Config: map[string]any{
								bodyKeyPrompt:   "Write an email to a hotel reception desk asking about room availability for an upcoming vacation. State your arrival and departure dates, mention the number of guests, and ask whether breakfast and airport shuttle service are included in the price.",
								cfgRubric:       "Write at least 60 words. Maintain a polite and professional tone throughout, specify reservation dates and guest count, and ask specific questions about breakfast and transport.",
								cfgMinWords:     60,
								cfgSampleAnswer: "Dear Reception Team,\n\nI am writing to inquire about room availability from July 12th to July 15th for two adults. We would like a double room with a city view if possible. Could you please confirm if complimentary breakfast and airport shuttle services are included in the room rate? Thank you for your assistance, and I look forward to your reply.\n\nSincerely,\nDavid Miller",
							},
							Body: map[string]any{
								bodyKeyPrompt:        "Write an email to a hotel reception desk asking about room availability for an upcoming vacation. State your arrival and departure dates, mention the number of guests, and ask whether breakfast and airport shuttle service are included in the price.",
								cfgRubric:            "Write at least 60 words. Maintain a polite and professional tone throughout, specify reservation dates and guest count, and ask specific questions about breakfast and transport.",
								cfgMinWords:          60,
								bodyKeyCorrectAnswer: "Dear Reception Team,\n\nI am writing to inquire about room availability from July 12th to July 15th for two adults. We would like a double room with a city view if possible. Could you please confirm if complimentary breakfast and airport shuttle services are included in the room rate? Thank you for your assistance, and I look forward to your reply.\n\nSincerely,\nDavid Miller",
								cfgSampleAnswer:      "Dear Reception Team,\n\nI am writing to inquire about room availability from July 12th to July 15th for two adults. We would like a double room with a city view if possible. Could you please confirm if complimentary breakfast and airport shuttle services are included in the room rate? Thank you for your assistance, and I look forward to your reply.\n\nSincerely,\nDavid Miller",
								bodyKeyExplanation: map[string]string{
									"text":    "Use formal salutations (Dear...), state your booking parameters clearly, ask concise questions, and close politely.",
									"text_vi": "Sử dụng lời mở đầu trang trọng (Dear...), nêu rõ ngày đến/đi và số lượng khách, đặt câu hỏi ngắn gọn về dịch vụ và kết thư lịch sự.",
								},
							},
						},
					},
				},
			},
		},
		{
			Position:    2,
			Title:       "Opinion & Argumentative Paragraphs (B1 - 120 words)",
			Description: "Express reasoned viewpoints on lifestyle, education, and community topics.",
			Lessons: []seedLesson{
				{
					Position:         1,
					Title:            "Living in Cities vs Small Towns",
					SkillFocus:       skillWriting,
					EstimatedMinutes: 20,
					Activities: []seedActivity{
						{
							Position: 1,
							Kind:     kindWritingPrompt,
							Config: map[string]any{
								bodyKeyPrompt:   "Some people prefer living in metropolitan cities, while others prefer the tranquility of small towns or rural areas. Write an opinion essay explaining which setting is better for young adults starting their careers. Provide clear reasons and examples.",
								cfgRubric:       "Write at least 120 words. State a clear opinion in the introduction, develop two distinct supporting arguments with examples, and write a concise concluding sentence.",
								cfgMinWords:     120,
								cfgSampleAnswer: "In my opinion, living in a metropolitan city is significantly more advantageous for young adults who are just embarking on their careers. First and foremost, major cities provide a much wider spectrum of job opportunities, professional networking events, and career advancement paths across modern industries such as technology, finance, and media. Furthermore, urban environments offer comprehensive public transit systems and abundant cultural amenities, including museums, international restaurants, and diverse social communities. Although housing and living expenses are undoubtedly higher than in small towns, the exposure to diverse perspectives and accelerated professional growth during one's twenties outweighs the financial drawbacks. Therefore, young professionals benefit greatly from beginning their working lives in dynamic urban centers.",
							},
							Body: map[string]any{
								bodyKeyPrompt:        "Some people prefer living in metropolitan cities, while others prefer the tranquility of small towns or rural areas. Write an opinion essay explaining which setting is better for young adults starting their careers. Provide clear reasons and examples.",
								cfgRubric:            "Write at least 120 words. State a clear opinion in the introduction, develop two distinct supporting arguments with examples, and write a concise concluding sentence.",
								cfgMinWords:          120,
								bodyKeyCorrectAnswer: "In my opinion, living in a metropolitan city is significantly more advantageous for young adults who are just embarking on their careers. First and foremost, major cities provide a much wider spectrum of job opportunities, professional networking events, and career advancement paths across modern industries such as technology, finance, and media. Furthermore, urban environments offer comprehensive public transit systems and abundant cultural amenities, including museums, international restaurants, and diverse social communities. Although housing and living expenses are undoubtedly higher than in small towns, the exposure to diverse perspectives and accelerated professional growth during one's twenties outweighs the financial drawbacks. Therefore, young professionals benefit greatly from beginning their working lives in dynamic urban centers.",
								cfgSampleAnswer:      "In my opinion, living in a metropolitan city is significantly more advantageous for young adults who are just embarking on their careers. First and foremost, major cities provide a much wider spectrum of job opportunities, professional networking events, and career advancement paths across modern industries such as technology, finance, and media. Furthermore, urban environments offer comprehensive public transit systems and abundant cultural amenities, including museums, international restaurants, and diverse social communities. Although housing and living expenses are undoubtedly higher than in small towns, the exposure to diverse perspectives and accelerated professional growth during one's twenties outweighs the financial drawbacks. Therefore, young professionals benefit greatly from beginning their working lives in dynamic urban centers.",
								bodyKeyExplanation: map[string]string{
									"text":    "Structure your essay with an introductory stance, points on employment and amenities, acknowledge cost of living, and conclude firmly.",
									"text_vi": "Xây dựng bài viết với mở đoạn rõ ràng, các ý bổ trợ về cơ hội việc làm và tiện ích, thừa nhận chi phí sống và kết luận khẳng định quan điểm.",
								},
							},
						},
					},
				},
				{
					Position:         2,
					Title:            "Mandatory Community Service for Students",
					SkillFocus:       skillWriting,
					EstimatedMinutes: 20,
					Activities: []seedActivity{
						{
							Position: 1,
							Kind:     kindWritingPrompt,
							Config: map[string]any{
								bodyKeyPrompt:   "Do you agree or disagree that secondary school students should be required to complete unpaid community service before graduation? Write an essay explaining your perspective with supporting arguments.",
								cfgRubric:       "Write at least 120 words. Clearly express your position, develop arguments on civic responsibility and practical skills, and address potential objections before concluding.",
								cfgMinWords:     120,
								cfgSampleAnswer: "I firmly agree that high school students should be required to participate in community service before receiving their diplomas. Engaging in volunteer activities, such as assisting at local food banks, tutoring younger pupils, or participating in environmental clean-up campaigns, instills valuable empathy and civic responsibility in young minds. Moreover, community service allows adolescents to step away from academic stress and acquire essential life skills, including teamwork, communication, and practical problem solving, which are rarely taught in standard classrooms. While critics may argue that mandatory service places excessive burdens on students already preparing for university entrance exams, allocating just twenty hours across the school year is thoroughly manageable. In conclusion, requiring voluntary community contributions fosters well-rounded citizens and strengthens our collective society.",
							},
							Body: map[string]any{
								bodyKeyPrompt:        "Do you agree or disagree that secondary school students should be required to complete unpaid community service before graduation? Write an essay explaining your perspective with supporting arguments.",
								cfgRubric:            "Write at least 120 words. Clearly express your position, develop arguments on civic responsibility and practical skills, and address potential objections before concluding.",
								cfgMinWords:          120,
								bodyKeyCorrectAnswer: "I firmly agree that high school students should be required to participate in community service before receiving their diplomas. Engaging in volunteer activities, such as assisting at local food banks, tutoring younger pupils, or participating in environmental clean-up campaigns, instills valuable empathy and civic responsibility in young minds. Moreover, community service allows adolescents to step away from academic stress and acquire essential life skills, including teamwork, communication, and practical problem solving, which are rarely taught in standard classrooms. While critics may argue that mandatory service places excessive burdens on students already preparing for university entrance exams, allocating just twenty hours across the school year is thoroughly manageable. In conclusion, requiring voluntary community contributions fosters well-rounded citizens and strengthens our collective society.",
								cfgSampleAnswer:      "I firmly agree that high school students should be required to participate in community service before receiving their diplomas. Engaging in volunteer activities, such as assisting at local food banks, tutoring younger pupils, or participating in environmental clean-up campaigns, instills valuable empathy and civic responsibility in young minds. Moreover, community service allows adolescents to step away from academic stress and acquire essential life skills, including teamwork, communication, and practical problem solving, which are rarely taught in standard classrooms. While critics may argue that mandatory service places excessive burdens on students already preparing for university entrance exams, allocating just twenty hours across the school year is thoroughly manageable. In conclusion, requiring voluntary community contributions fosters well-rounded citizens and strengthens our collective society.",
								bodyKeyExplanation: map[string]string{
									"text":    "State your position clearly, provide tangible volunteer examples, rebut the objection about time constraints, and conclude.",
									"text_vi": "Khẳng định lập trường dứt khoát, đưa ví dụ tình nguyện thực tế, giải quyết phản biện về áp lực thời gian và tóm tắt kết luận.",
								},
							},
						},
					},
				},
			},
		},
		{
			Position:    3,
			Title:       "Academic Argumentative Essays (B2 IELTS Task 2 shape - 250 words)",
			Description: "Write comprehensive four-paragraph balanced argumentative essays on global science and society dilemmas.",
			Lessons: []seedLesson{
				{
					Position:         1,
					Title:            "Space Exploration vs Earth Priorities",
					SkillFocus:       skillWriting,
					EstimatedMinutes: 30,
					Activities: []seedActivity{
						{
							Position: 1,
							Kind:     kindWritingPrompt,
							Config: map[string]any{
								bodyKeyPrompt:   "In many nations, astronomical budgets are allocated to space exploration missions. Some citizens believe this expenditure is unjustified and that public funds should instead address urgent terrestrial challenges such as poverty, healthcare, and climate change. Discuss both views and give your own opinion.",
								cfgRubric:       "Write at least 250 words in an IELTS Task 2 balanced essay format. Introduce both viewpoints, dedicate one body paragraph to terrestrial priorities, dedicate a second body paragraph to technological spin-offs of space research, and conclude with your personal verdict.",
								cfgMinWords:     250,
								cfgSampleAnswer: "The allocation of public funding toward space exploration has long sparked intense global debate. While proponents argue that space missions drive scientific innovation and expand human knowledge, critics contend that such enormous expenditures are irresponsible given the pressing socio-economic crises currently afflicting our planet. This essay will examine both perspectives before presenting my own view.\n\nOn the one hand, opponents of aerospace investment argue that governments possess a moral obligation to prioritize immediate human welfare. Millions of people across the globe suffer from severe poverty, inadequate healthcare infrastructure, and food insecurity, while rising global temperatures threaten coastal communities with ecological disaster. From this standpoint, diverting billions of taxpayer dollars toward interplanetary expeditions appears deeply misplaced when basic human necessities remain unfulfilled on Earth. Resolving global malnutrition and transitioning to renewable energy systems undoubtedly require massive capital that could be directly sourced from space exploration budgets.\n\nOn the other hand, advocates emphasize that space research yields profound technological breakthroughs that directly benefit earthly life. Innovations originally engineered for space programs—such as satellite communication, advanced weather forecasting, water purification membranes, and lightweight materials—have transformed modern civilization and global commerce. Furthermore, climate monitoring satellites are indispensable tools for tracking deforestation and predicting severe weather events, thereby actively mitigating environmental damage.\n\nIn conclusion, although addressing immediate terrestrial hardships must remain a primary government responsibility, abandoning space exploration would be short-sighted. I believe governments should pursue a balanced fiscal approach, investing responsibly in both urgent humanitarian relief and strategic scientific frontiers that secure our collective long-term future.",
							},
							Body: map[string]any{
								bodyKeyPrompt:        "In many nations, astronomical budgets are allocated to space exploration missions. Some citizens believe this expenditure is unjustified and that public funds should instead address urgent terrestrial challenges such as poverty, healthcare, and climate change. Discuss both views and give your own opinion.",
								cfgRubric:            "Write at least 250 words in an IELTS Task 2 balanced essay format. Introduce both viewpoints, dedicate one body paragraph to terrestrial priorities, dedicate a second body paragraph to technological spin-offs of space research, and conclude with your personal verdict.",
								cfgMinWords:          250,
								bodyKeyCorrectAnswer: "The allocation of public funding toward space exploration has long sparked intense global debate. While proponents argue that space missions drive scientific innovation and expand human knowledge, critics contend that such enormous expenditures are irresponsible given the pressing socio-economic crises currently afflicting our planet. This essay will examine both perspectives before presenting my own view.\n\nOn the one hand, opponents of aerospace investment argue that governments possess a moral obligation to prioritize immediate human welfare. Millions of people across the globe suffer from severe poverty, inadequate healthcare infrastructure, and food insecurity, while rising global temperatures threaten coastal communities with ecological disaster. From this standpoint, diverting billions of taxpayer dollars toward interplanetary expeditions appears deeply misplaced when basic human necessities remain unfulfilled on Earth. Resolving global malnutrition and transitioning to renewable energy systems undoubtedly require massive capital that could be directly sourced from space exploration budgets.\n\nOn the other hand, advocates emphasize that space research yields profound technological breakthroughs that directly benefit earthly life. Innovations originally engineered for space programs—such as satellite communication, advanced weather forecasting, water purification membranes, and lightweight materials—have transformed modern civilization and global commerce. Furthermore, climate monitoring satellites are indispensable tools for tracking deforestation and predicting severe weather events, thereby actively mitigating environmental damage.\n\nIn conclusion, although addressing immediate terrestrial hardships must remain a primary government responsibility, abandoning space exploration would be short-sighted. I believe governments should pursue a balanced fiscal approach, investing responsibly in both urgent humanitarian relief and strategic scientific frontiers that secure our collective long-term future.",
								cfgSampleAnswer:      "The allocation of public funding toward space exploration has long sparked intense global debate. While proponents argue that space missions drive scientific innovation and expand human knowledge, critics contend that such enormous expenditures are irresponsible given the pressing socio-economic crises currently afflicting our planet. This essay will examine both perspectives before presenting my own view.\n\nOn the one hand, opponents of aerospace investment argue that governments possess a moral obligation to prioritize immediate human welfare. Millions of people across the globe suffer from severe poverty, inadequate healthcare infrastructure, and food insecurity, while rising global temperatures threaten coastal communities with ecological disaster. From this standpoint, diverting billions of taxpayer dollars toward interplanetary expeditions appears deeply misplaced when basic human necessities remain unfulfilled on Earth. Resolving global malnutrition and transitioning to renewable energy systems undoubtedly require massive capital that could be directly sourced from space exploration budgets.\n\nOn the other hand, advocates emphasize that space research yields profound technological breakthroughs that directly benefit earthly life. Innovations originally engineered for space programs—such as satellite communication, advanced weather forecasting, water purification membranes, and lightweight materials—have transformed modern civilization and global commerce. Furthermore, climate monitoring satellites are indispensable tools for tracking deforestation and predicting severe weather events, thereby actively mitigating environmental damage.\n\nIn conclusion, although addressing immediate terrestrial hardships must remain a primary government responsibility, abandoning space exploration would be short-sighted. I believe governments should pursue a balanced fiscal approach, investing responsibly in both urgent humanitarian relief and strategic scientific frontiers that secure our collective long-term future.",
								bodyKeyExplanation: map[string]string{
									"text":    "An effective IELTS Task 2 discusses both views in balanced body paragraphs before providing an informed synthesis in the conclusion.",
									"text_vi": "Bài luận chuẩn IELTS Task 2 thảo luận cân bằng hai quan điểm ở 2 đoạn thân bài trước khi đưa ra nhận định tổng hợp trong phần kết bài.",
								},
							},
						},
					},
				},
				{
					Position:         2,
					Title:            "Artificial Intelligence and Employment",
					SkillFocus:       skillWriting,
					EstimatedMinutes: 30,
					Activities: []seedActivity{
						{
							Position: 1,
							Kind:     kindWritingPrompt,
							Config: map[string]any{
								bodyKeyPrompt:   "The accelerated development of artificial intelligence and machine learning technologies is rapidly reshaping the global labor market. While some experts predict that automation will trigger unprecedented unemployment, others maintain that it will generate superior career opportunities and foster economic growth. Discuss both views and state your personal opinion.",
								cfgRubric:       "Write at least 250 words. Present balanced perspectives on employment displacement versus economic expansion, and articulate a clear conclusion emphasizing workforce adaptability.",
								cfgMinWords:     250,
								cfgSampleAnswer: "The exponential advancement of artificial intelligence and automated systems has ignited widespread deliberation regarding the future landscape of human labor. While pessimists predict that intelligent algorithms will inevitably displace millions of white-collar and manual workers, optimists argue that technological revolutions historically generate more employment opportunities than they eliminate. This essay will explore both arguments before offering my conclusion.\n\nThere is considerable justification for apprehension regarding technological unemployment. Routine cognitive and physical tasks—such as administrative data processing, basic financial accounting, customer support, and assembly-line manufacturing—are increasingly performed by automated software and robotics with greater precision and substantially lower operational costs. As these technologies mature, workers in vulnerable sectors face imminent redundancy unless they undergo comprehensive retraining, which may prove difficult for older demographics. The speed of contemporary digital transformation threatens to outpace the rate at which human workers can adapt, potentially exacerbating income inequality across developed and developing economies.\n\nConversely, proponents maintain that artificial intelligence acts as a powerful catalyst for economic expansion and novel industry creation. Historical precedents, including the Industrial Revolution and the advent of the personal computer, demonstrate that technological displacement ultimately gives rise to entirely new professions. AI systems enhance human productivity by automating tedious administrative tasks, allowing professionals to concentrate on strategic decision-making, creative expression, and empathetic communication. Moreover, the burgeoning technology sector generates robust demand for data scientists, cybersecurity specialists, and ethical compliance analysts.\n\nTo conclude, while artificial intelligence will undoubtedly disrupt existing labor dynamics in the near term, I believe it will ultimately enrich the global economy. Governments and educational institutions must proactively institute lifelong learning frameworks and vocational retraining initiatives to ensure that the workforce thrives alongside intelligent machines.",
							},
							Body: map[string]any{
								bodyKeyPrompt:        "The accelerated development of artificial intelligence and machine learning technologies is rapidly reshaping the global labor market. While some experts predict that automation will trigger unprecedented unemployment, others maintain that it will generate superior career opportunities and foster economic growth. Discuss both views and state your personal opinion.",
								cfgRubric:            "Write at least 250 words. Present balanced perspectives on employment displacement versus economic expansion, and articulate a clear conclusion emphasizing workforce adaptability.",
								cfgMinWords:          250,
								bodyKeyCorrectAnswer: "The exponential advancement of artificial intelligence and automated systems has ignited widespread deliberation regarding the future landscape of human labor. While pessimists predict that intelligent algorithms will inevitably displace millions of white-collar and manual workers, optimists argue that technological revolutions historically generate more employment opportunities than they eliminate. This essay will explore both arguments before offering my conclusion.\n\nThere is considerable justification for apprehension regarding technological unemployment. Routine cognitive and physical tasks—such as administrative data processing, basic financial accounting, customer support, and assembly-line manufacturing—are increasingly performed by automated software and robotics with greater precision and substantially lower operational costs. As these technologies mature, workers in vulnerable sectors face imminent redundancy unless they undergo comprehensive retraining, which may prove difficult for older demographics. The speed of contemporary digital transformation threatens to outpace the rate at which human workers can adapt, potentially exacerbating income inequality across developed and developing economies.\n\nConversely, proponents maintain that artificial intelligence acts as a powerful catalyst for economic expansion and novel industry creation. Historical precedents, including the Industrial Revolution and the advent of the personal computer, demonstrate that technological displacement ultimately gives rise to entirely new professions. AI systems enhance human productivity by automating tedious administrative tasks, allowing professionals to concentrate on strategic decision-making, creative expression, and empathetic communication. Moreover, the burgeoning technology sector generates robust demand for data scientists, cybersecurity specialists, and ethical compliance analysts.\n\nTo conclude, while artificial intelligence will undoubtedly disrupt existing labor dynamics in the near term, I believe it will ultimately enrich the global economy. Governments and educational institutions must proactively institute lifelong learning frameworks and vocational retraining initiatives to ensure that the workforce thrives alongside intelligent machines.",
								cfgSampleAnswer:      "The exponential advancement of artificial intelligence and automated systems has ignited widespread deliberation regarding the future landscape of human labor. While pessimists predict that intelligent algorithms will inevitably displace millions of white-collar and manual workers, optimists argue that technological revolutions historically generate more employment opportunities than they eliminate. This essay will explore both arguments before offering my conclusion.\n\nThere is considerable justification for apprehension regarding technological unemployment. Routine cognitive and physical tasks—such as administrative data processing, basic financial accounting, customer support, and assembly-line manufacturing—are increasingly performed by automated software and robotics with greater precision and substantially lower operational costs. As these technologies mature, workers in vulnerable sectors face imminent redundancy unless they undergo comprehensive retraining, which may prove difficult for older demographics. The speed of contemporary digital transformation threatens to outpace the rate at which human workers can adapt, potentially exacerbating income inequality across developed and developing economies.\n\nConversely, proponents maintain that artificial intelligence acts as a powerful catalyst for economic expansion and novel industry creation. Historical precedents, including the Industrial Revolution and the advent of the personal computer, demonstrate that technological displacement ultimately gives rise to entirely new professions. AI systems enhance human productivity by automating tedious administrative tasks, allowing professionals to concentrate on strategic decision-making, creative expression, and empathetic communication. Moreover, the burgeoning technology sector generates robust demand for data scientists, cybersecurity specialists, and ethical compliance analysts.\n\nTo conclude, while artificial intelligence will undoubtedly disrupt existing labor dynamics in the near term, I believe it will ultimately enrich the global economy. Governments and educational institutions must proactively institute lifelong learning frameworks and vocational retraining initiatives to ensure that the workforce thrives alongside intelligent machines.",
								bodyKeyExplanation: map[string]string{
									"text":    "Discuss the risks of displacement alongside historical productivity growth and the emergence of new high-skill occupations.",
									"text_vi": "Phân tích rủi ro mất việc làm bên cạnh tăng trưởng năng suất lịch sử và sự ra đời của các nhóm ngành nghề chất lượng cao mới.",
								},
							},
						},
					},
				},
			},
		},
	},
}
