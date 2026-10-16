package chat

import (
	"reflect"
	"testing"
)

func TestParseOfferText(t *testing.T) {
	type want struct {
		typ, sub, race, seal string
		degree, plus         int
	}
	cases := []struct {
		text string
		want []want
	}{
		{"WTB shield sun 12d euro pm me !", []want{{"shields", "", "european", "sun", 12, 0}}},
		{"WTB STAFF MOON D 12 OR Star D13 For Silk WTS silk", []want{{"weapons", "staff", "", "moon", 12, 0}, {"weapons", "staff", "", "star", 13, 0}}},
		{"WTB 13D CH SWORD SUN OR MOON", []want{{"weapons", "sword_ch", "chinese", "sun", 13, 0}, {"weapons", "sword_ch", "chinese", "moon", 13, 0}}},
		{"WTB robe 11d nova and wtb 11d magic stones pm", []want{{"sets", "garment", "european", "nova", 11, 0}, {"alchemy", "magic_stone", "", "", 11, 0}}},
		{"WTB STAFF D13 +15 FOR SILK", []want{{"weapons", "staff", "", "", 13, 15}}},
		{"WTB 1 ring euro 13d star", []want{{"accessories", "ring", "european", "star", 13, 0}}},
		{"WTB germatn foot chest legs sun", nil},
		{"WTS hello how are you", nil},
	}
	for _, c := range cases {
		items := ParseOfferText(c.text)
		var got []want
		for _, item := range items {
			got = append(got, want{item.Type, item.Subtype, item.Race, item.Seal, item.Degree, item.Plus})
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q = %+v, want %+v", c.text, got, c.want)
		}
	}
	items := ParseOfferText("WTB clean staff d13 for 50s")
	if len(items) != 1 || items[0].Price != "50s" || items[0].Currency != "" || len(items[0].Notes) != 1 || items[0].Notes[0] != "clean" {
		t.Errorf("price/notes = %+v", items)
	}
	if items := ParseOfferText("WTS staff d13 for silk"); len(items) != 1 || items[0].Currency != "silk" {
		t.Errorf("currency = %+v", items)
	}
}
