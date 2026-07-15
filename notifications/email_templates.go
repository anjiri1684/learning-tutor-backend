package notifications

import "fmt"

const emailWrapper = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:#0a0a0a;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif">
<table width="100%%" cellpadding="0" cellspacing="0" style="background-color:#0a0a0a;padding:40px 0">
<tr><td align="center">
<table width="600" cellpadding="0" cellspacing="0" style="background-color:#111111;border-radius:16px;overflow:hidden;border:1px solid #1e1e1e">
  <!-- Header -->
  <tr>
    <td style="background:linear-gradient(135deg,#7c3aed,#a855f7);padding:32px 40px;text-align:center">
      <p style="margin:0;font-size:11px;font-weight:600;letter-spacing:3px;color:rgba(255,255,255,0.6);text-transform:uppercase">PHY International</p>
      <h1 style="margin:8px 0 0;font-size:22px;font-weight:700;color:#ffffff;letter-spacing:-0.3px">Language Learning Centre</h1>
    </td>
  </tr>
  <!-- Content -->
  <tr>
    <td style="padding:36px 40px">
      %s
    </td>
  </tr>
  <!-- Footer -->
  <tr>
    <td style="padding:24px 40px;background-color:#0d0d0d;border-top:1px solid #1e1e1e;text-align:center">
      <p style="margin:0 0 8px;font-size:12px;color:#525252">© 2026 PHY International Language Learning Centre. All rights reserved.</p>
      <p style="margin:0;font-size:11px;color:#3a3a3a">Empowering learners worldwide through expert language instruction.</p>
    </td>
  </tr>
</table>
</td></tr>
</table>
</body>
</html>`

// ── Registration ──────────────────────────────────────────

func WelcomeTemplate(name string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Welcome, %s!</h2>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Your account has been created successfully. You&rsquo;re now part of a global community of language learners.</p>
<table width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:24px">
<tr><td style="padding:16px;background-color:#1a1a1a;border-radius:10px;border:1px solid #252525">
  <p style="margin:0 0 12px;font-size:14px;font-weight:600;color:#a855f7;letter-spacing:0.5px">GETTING STARTED</p>
  <table width="100%%" cellpadding="0" cellspacing="0">
    <tr><td style="padding:6px 0;font-size:13px;color:#d4d4d4"><span style="color:#a855f7;font-weight:600">1.</span>&nbsp; Browse expert tutors &amp; pick your language</td></tr>
    <tr><td style="padding:6px 0;font-size:13px;color:#d4d4d4"><span style="color:#a855f7;font-weight:600">2.</span>&nbsp; Book a session at a time that suits you</td></tr>
    <tr><td style="padding:6px 0;font-size:13px;color:#d4d4d4"><span style="color:#a855f7;font-weight:600">3.</span>&nbsp; Start speaking with confidence!</td></tr>
  </table>
</td></tr>
</table>
<a href="https://phylanguagecenter.com/dashboard/find-teachers" style="display:inline-block;background:linear-gradient(135deg,#7c3aed,#a855f7);color:#ffffff;text-decoration:none;padding:14px 32px;border-radius:10px;font-size:14px;font-weight:600;letter-spacing:0.2px">Find a Tutor</a>
`, name)
	return "Welcome to PHY Language Centre!", fmt.Sprintf(emailWrapper, body)
}

// ── Password Reset ────────────────────────────────────────

func PasswordResetTemplate(name, resetLink string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Reset Your Password</h2>
<p style="margin:0 0 8px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s,</p>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">We received a request to reset the password for your account. Click the button below to set a new password. This link expires in <strong style="color:#d4d4d4">15 minutes</strong>.</p>
<a href="%s" style="display:inline-block;background:linear-gradient(135deg,#7c3aed,#a855f7);color:#ffffff;text-decoration:none;padding:14px 32px;border-radius:10px;font-size:14px;font-weight:600;letter-spacing:0.2px">Reset Password</a>
<p style="margin:24px 0 0;font-size:13px;color:#525252">If you did not request a password reset, you can safely ignore this email.</p>
`, name, resetLink)
	return "Reset Your Password", fmt.Sprintf(emailWrapper, body)
}

// ── Booking Confirmed ─────────────────────────────────────

func BookingConfirmedStudentTemplate(name string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Booking Confirmed!</h2>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s, your payment was successful and your class is confirmed. Your tutor will share the meeting link before the session.</p>
<table width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:24px">
<tr><td style="padding:14px 18px;background-color:#1a1a1a;border-radius:10px;border:1px solid #1e3a1e">
  <p style="margin:0;font-size:13px;color:#4ade80;font-weight:600">&#10003; Payment confirmed &mdash; you&rsquo;re all set!</p>
</td></tr>
</table>
<a href="https://phylanguagecenter.com/dashboard/my-classes" style="display:inline-block;background:linear-gradient(135deg,#7c3aed,#a855f7);color:#ffffff;text-decoration:none;padding:14px 32px;border-radius:10px;font-size:14px;font-weight:600;letter-spacing:0.2px">View My Classes</a>
`, name)
	return "Your Booking is Confirmed!", fmt.Sprintf(emailWrapper, body)
}

func BookingConfirmedTeacherTemplate(name string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">New Booking</h2>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s, a student has booked and paid for a session with you. Please check your dashboard and prepare for the class.</p>
<table width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:24px">
<tr><td style="padding:14px 18px;background-color:#1a1a1a;border-radius:10px;border:1px solid #1e1a3a">
  <p style="margin:0;font-size:13px;color:#a78bfa;font-weight:600">&#128214; Session paid &mdash; prepare your materials</p>
</td></tr>
</table>
<a href="https://phylanguagecenter.com/teacher/classes" style="display:inline-block;background:linear-gradient(135deg,#7c3aed,#a855f7);color:#ffffff;text-decoration:none;padding:14px 32px;border-radius:10px;font-size:14px;font-weight:600;letter-spacing:0.2px">View Dashboard</a>
`, name)
	return "You Have a New Booking!", fmt.Sprintf(emailWrapper, body)
}

// ── Credit Booking Confirmed ──────────────────────────────

func CreditBookingConfirmedStudentTemplate(name string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Class Booked with Credits</h2>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s, your class has been booked using your credit balance. No payment was required.</p>
<a href="https://phylanguagecenter.com/dashboard/my-classes" style="display:inline-block;background:linear-gradient(135deg,#7c3aed,#a855f7);color:#ffffff;text-decoration:none;padding:14px 32px;border-radius:10px;font-size:14px;font-weight:600;letter-spacing:0.2px">View My Classes</a>
`, name)
	return "Your Booking is Confirmed!", fmt.Sprintf(emailWrapper, body)
}

func CreditBookingConfirmedTeacherTemplate(name string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">New Booking (Credit)</h2>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s, a student has booked a session with you using their credit balance.</p>
<a href="https://phylanguagecenter.com/teacher/classes" style="display:inline-block;background:linear-gradient(135deg,#7c3aed,#a855f7);color:#ffffff;text-decoration:none;padding:14px 32px;border-radius:10px;font-size:14px;font-weight:600;letter-spacing:0.2px">View Dashboard</a>
`, name)
	return "You Have a New Booking!", fmt.Sprintf(emailWrapper, body)
}

// ── Meeting Link ──────────────────────────────────────────

func MeetingLinkTemplate(name, meetingLink string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Your Meeting Link is Ready</h2>
<p style="margin:0 0 8px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s,</p>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Here is the link for your upcoming class:</p>
<table width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:24px">
<tr><td style="padding:16px 20px;background-color:#1a1a1a;border-radius:10px;border:1px solid #252525;word-break:break-all">
  <p style="margin:0;font-size:13px;color:#a3a3a3"><a href="%s" style="color:#a855f7;font-weight:600">%s</a></p>
</td></tr>
</table>
<a href="%s" style="display:inline-block;background:linear-gradient(135deg,#7c3aed,#a855f7);color:#ffffff;text-decoration:none;padding:14px 32px;border-radius:10px;font-size:14px;font-weight:600;letter-spacing:0.2px">Join Class</a>
`, name, meetingLink, meetingLink, meetingLink)
	return "Your Meeting Link is Here!", fmt.Sprintf(emailWrapper, body)
}

// ── Class Reminder ────────────────────────────────────────

func ClassReminderTemplate(name, startTime, meetingLink string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Class Starts in 1 Hour</h2>
<p style="margin:0 0 8px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s,</p>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Your class is scheduled to start at <strong style="color:#d4d4d4">%s</strong>. Please join using the link below.</p>
<a href="%s" style="display:inline-block;background:linear-gradient(135deg,#7c3aed,#a855f7);color:#ffffff;text-decoration:none;padding:14px 32px;border-radius:10px;font-size:14px;font-weight:600;letter-spacing:0.2px">Join Class</a>
`, name, startTime, meetingLink)
	return "Reminder: Your Class Starts in 1 Hour!", fmt.Sprintf(emailWrapper, body)
}

// ── Bundle Purchase ───────────────────────────────────────

func BundlePurchasedTemplate(name string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Bundle Purchase Confirmed!</h2>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s, your class bundle purchase is complete. Your credits are now active and ready to use for booking sessions.</p>
<table width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:24px">
<tr><td style="padding:14px 18px;background-color:#1a1a1a;border-radius:10px;border:1px solid #1e3a1e">
  <p style="margin:0;font-size:13px;color:#4ade80;font-weight:600">&#10003; Credits ready &mdash; book your next session now</p>
</td></tr>
</table>
<a href="https://phylanguagecenter.com/dashboard/my-bundles" style="display:inline-block;background:linear-gradient(135deg,#7c3aed,#a855f7);color:#ffffff;text-decoration:none;padding:14px 32px;border-radius:10px;font-size:14px;font-weight:600;letter-spacing:0.2px">View My Bundles</a>
`, name)
	return "Bundle Purchase Confirmed!", fmt.Sprintf(emailWrapper, body)
}

func CreditBundlePurchasedTemplate(name string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Bundle Purchased with Credits</h2>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s, your class bundle has been purchased using your credit balance and is now active.</p>
<a href="https://phylanguagecenter.com/dashboard/my-bundles" style="display:inline-block;background:linear-gradient(135deg,#7c3aed,#a855f7);color:#ffffff;text-decoration:none;padding:14px 32px;border-radius:10px;font-size:14px;font-weight:600;letter-spacing:0.2px">View My Bundles</a>
`, name)
	return "Bundle Purchase Confirmed!", fmt.Sprintf(emailWrapper, body)
}

// ── Teacher Application ───────────────────────────────────

func TeacherApplicationApprovedTemplate(name string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Application Approved!</h2>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Congratulations, %s! Your application to become a teacher has been approved. You can now set your availability and start accepting students.</p>
<a href="https://phylanguagecenter.com/teacher/availability" style="display:inline-block;background:linear-gradient(135deg,#7c3aed,#a855f7);color:#ffffff;text-decoration:none;padding:14px 32px;border-radius:10px;font-size:14px;font-weight:600;letter-spacing:0.2px">Set Your Availability</a>
`, name)
	return "Your Teacher Application has been Approved!", fmt.Sprintf(emailWrapper, body)
}

func TeacherApplicationRejectedTemplate(name string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Application Update</h2>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s, after careful review, your teacher application was not approved at this time. Feel free to reapply in the future.</p>
`, name)
	return "Update on Your Teacher Application", fmt.Sprintf(emailWrapper, body)
}

// ── Reschedule ────────────────────────────────────────────

func RescheduleRequestTemplate(teacherName string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Reschedule Request</h2>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s, a student has requested to reschedule a class. Please log in to review and approve or deny the request.</p>
<a href="https://phylanguagecenter.com/teacher/reschedules" style="display:inline-block;background:linear-gradient(135deg,#7c3aed,#a855f7);color:#ffffff;text-decoration:none;padding:14px 32px;border-radius:10px;font-size:14px;font-weight:600;letter-spacing:0.2px">Review Request</a>
`, teacherName)
	return "Reschedule Request", fmt.Sprintf(emailWrapper, body)
}

func RescheduleApprovedTemplate(name string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Reschedule Approved</h2>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s, your request to reschedule the class has been approved by the teacher. Check your dashboard for the updated time.</p>
<a href="https://phylanguagecenter.com/dashboard/my-classes" style="display:inline-block;background:linear-gradient(135deg,#7c3aed,#a855f7);color:#ffffff;text-decoration:none;padding:14px 32px;border-radius:10px;font-size:14px;font-weight:600;letter-spacing:0.2px">View My Classes</a>
`, name)
	return "Reschedule Approved", fmt.Sprintf(emailWrapper, body)
}

func RescheduleRejectedTemplate(name string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Reschedule Not Approved</h2>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s, your request to reschedule the class was not approved by the teacher. The session remains as originally scheduled.</p>
<a href="https://phylanguagecenter.com/dashboard/my-classes" style="display:inline-block;background:linear-gradient(135deg,#7c3aed,#a855f7);color:#ffffff;text-decoration:none;padding:14px 32px;border-radius:10px;font-size:14px;font-weight:600;letter-spacing:0.2px">View My Classes</a>
`, name)
	return "Reschedule Not Approved", fmt.Sprintf(emailWrapper, body)
}

// ── Refund ────────────────────────────────────────────────

func RefundProcessedTemplate(name string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Refund Processed</h2>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s, your refund request has been approved and processed by our team. The amount has been credited back to your account.</p>
`, name)
	return "Your Refund has been Processed", fmt.Sprintf(emailWrapper, body)
}

func RefundRejectedTemplate(name string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Refund Request Update</h2>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s, your refund request has been reviewed and was not approved at this time.</p>
`, name)
	return "Update on Your Refund Request", fmt.Sprintf(emailWrapper, body)
}

// ── Payout ────────────────────────────────────────────────

func PayoutProcessedTemplate(name string, amount float64) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Payout Processed</h2>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s, your payout request for <strong style="color:#d4d4d4">$%.2f</strong> has been processed and sent by our team.</p>
`, name, amount)
	return "Your Payout Has Been Processed", fmt.Sprintf(emailWrapper, body)
}

func PayoutRejectedTemplate(name string, amount float64, notes string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Payout Request Update</h2>
<p style="margin:0 0 16px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s, your payout request for <strong style="color:#d4d4d4">$%.2f</strong> was rejected. The funds have been returned to your account balance.</p>
<table width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:24px">
<tr><td style="padding:14px 18px;background-color:#1a1a1a;border-radius:10px;border:1px solid #252525">
  <p style="margin:0 0 6px;font-size:12px;color:#737373;font-weight:600;letter-spacing:0.5px">ADMIN NOTES</p>
  <p style="margin:0;font-size:13px;color:#d4d4d4">%s</p>
</td></tr>
</table>
`, name, amount, notes)
	return "Update on Your Payout Request", fmt.Sprintf(emailWrapper, body)
}

// ── Referral ──────────────────────────────────────────────

func ReferralEarnedTemplate(name string) (subject, html string) {
	body := fmt.Sprintf(`
<h2 style="margin:0 0 8px;font-size:20px;color:#ffffff">Referral Credit Earned!</h2>
<p style="margin:0 0 24px;font-size:15px;color:#a3a3a3;line-height:1.6">Hi %s, someone you referred has made their first purchase. A <strong style="color:#4ade80">$5.00 credit</strong> has been added to your account balance.</p>
<a href="https://phylanguagecenter.com/dashboard/profile" style="display:inline-block;background:linear-gradient(135deg,#7c3aed,#a855f7);color:#ffffff;text-decoration:none;padding:14px 32px;border-radius:10px;font-size:14px;font-weight:600;letter-spacing:0.2px">View My Balance</a>
`, name)
	return "You've Earned a Referral Credit!", fmt.Sprintf(emailWrapper, body)
}
