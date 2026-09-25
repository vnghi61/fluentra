package service

import "math/rand/v2"

// examTopics are the subjects an exam item is written about, one drawn at
// random per item. Without one the model returned the same two reading
// passages (night trains, urban beekeeping) six times over, so a composed test
// read three texts on one subject. The list mixes the workplace (TOEIC), the
// academic (IELTS) and everyday life (VSTEP), and avoids subjects a learner
// could be upset by.
var examTopics = []string{
	"a company moving to a new office building",
	"hiring and training new staff at a small firm",
	"a product launch that was delayed",
	"customer complaints at a hotel",
	"planning a trade fair or conference",
	"a supplier changing its delivery schedule",
	"working from home and hybrid offices",
	"a restaurant changing its menu",
	"an airline updating its baggage rules",
	"a bank opening a digital-only branch",
	"repairs to a public swimming pool",
	"a city introducing bike-sharing",
	"a museum reopening after renovation",
	"a local farmers' market",
	"volunteering at an animal shelter",
	"learning a musical instrument as an adult",
	"a community theatre production",
	"a family moving to a new city",
	"renting a first flat",
	"saving money for a holiday",
	"the history of the printing press",
	"how glaciers shape landscapes",
	"the migration of Arctic terns",
	"coral reefs and ocean temperature",
	"the invention of refrigeration",
	"why some languages disappear",
	"how sleep affects memory",
	"early human use of fire",
	"the science of weather forecasting",
	"desert plants that survive without rain",
	"how bridges are designed to resist wind",
	"the economics of fast fashion",
	"traditional crafts in the digital age",
	"the development of the bicycle",
	"how children learn to read",
	"the design of public libraries",
	"recycling electronic waste",
	"the spread of tea as a drink around the world",
	"volcanoes and the soil around them",
	"the architecture of ancient Rome",
	"space telescopes and distant planets",
	"the psychology of first impressions",
	"food waste in supermarkets",
	"the rise of electric buses",
	"rice farming in river deltas",
	"the role of bees and other pollinators in farming",
	"street food cultures in Asian cities",
	"how maps were made before satellites",
	"noise pollution in cities",
	"the benefits of learning a second language",
	"a school sports day",
	"a weekend camping trip",
	"choosing a university course",
	"a new public park opening",
	"a problem with an online order",
	"a guided tour of a factory",
	"a health clinic changing its opening hours",
	"a science museum's summer programme",
	"a cooking class for beginners",
	"a lost property office at a railway station",
}

// examTopic draws the subject for one exam item.
func examTopic() string {
	return examTopics[rand.IntN(len(examTopics))] //nolint:gosec // a topic draw, not a secret
}
